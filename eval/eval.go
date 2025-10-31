package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/api"
	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
)

var (
	// Private error variables (users don't need to check these)
	errEval         = errors.New("eval error")
	errScorer       = errors.New("scorer error")
	errTaskRun      = errors.New("task run error")
	errCaseIterator = errors.New("case iterator error")
)

var (
	// braintrust "span_attributes" for each type of eval span.
	evalSpanAttrs  = map[string]any{"type": "eval"}
	taskSpanAttrs  = map[string]any{"type": "task"}
	scoreSpanAttrs = map[string]any{"type": "score"}
)

// eval (private) is the execution engine for evaluations.
// It is created by newEval() and run via Run().
type eval[I, R any] struct {
	config         *config.Config
	session        *auth.Session
	experimentID   string
	experimentName string
	projectID      string
	projectName    string
	cases          Cases[I, R]
	task           Task[I, R]
	scorers        []Scorer[I, R]
	tracer         oteltrace.Tracer
	startSpanOpt   oteltrace.SpanStartOption
	goroutines     int
	quiet          bool
}

// nextCase is a wrapper for sending cases through a channel.
type nextCase[I, R any] struct {
	c       Case[I, R]
	iterErr error
}

// newEval creates a new eval executor with dependency injection.
// This replaces the old New() constructor which used global state.
func newEval[I, R any](ctx context.Context, cfg *config.Config, session *auth.Session, tp *trace.TracerProvider, opts Opts[I, R]) (*eval[I, R], error) {
	// Register/get experiment
	exp, err := registerExperiment(ctx, cfg, session, opts.Experiment, opts.Tags, opts.Metadata, opts.Update)
	if err != nil {
		return nil, fmt.Errorf("failed to register experiment: %w", err)
	}

	// Get project info for Result
	projectName := cfg.DefaultProjectName
	if projectName == "" {
		projectName = "unknown"
	}

	// Get project ID (registerExperiment already called RegisterProject)
	project, _ := api.RegisterProject(ctx, cfg, session, projectName)
	projectID := ""
	if project != nil {
		projectID = project.ID
	}

	// Create tracer from injected TracerProvider (instead of global)
	tracer := tp.Tracer("braintrust.eval")

	// Build parent span option
	parentAttr := fmt.Sprintf("experiment_id:%s", exp.ID)
	startSpanOpt := oteltrace.WithAttributes(attribute.String("braintrust.parent", parentAttr))

	// Set parallelism
	goroutines := opts.Parallelism
	if goroutines < 1 {
		goroutines = 1
	}

	return &eval[I, R]{
		config:         cfg,
		session:        session,
		experimentID:   exp.ID,
		experimentName: exp.Name,
		projectID:      projectID,
		projectName:    projectName,
		cases:          opts.Cases,
		task:           opts.Task,
		scorers:        opts.Scorers,
		tracer:         tracer,
		startSpanOpt:   startSpanOpt,
		goroutines:     goroutines,
		quiet:          opts.Quiet,
	}, nil
}

// run executes the evaluation with parallelism support.
// This is copied from the old Eval.Run() method.
func (e *eval[I, R]) run(ctx context.Context) (*Result, error) {
	start := time.Now()
	if e.experimentID == "" {
		return nil, fmt.Errorf("%w: experiment ID is required", errEval)
	}

	// Scale buffer size with parallelism to avoid blocking, but cap at 100
	bufferSize := minInt(e.goroutines*2, 100)
	nextCases := make(chan nextCase[I, R], bufferSize)
	var errs lockedErrors

	// Spawn our goroutines to run the cases.
	var wg sync.WaitGroup
	for i := 0; i < e.goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				nextCase, ok := <-nextCases
				if !ok {
					return
				}
				if err := e.runNextCase(ctx, nextCase); err != nil {
					errs.append(err)
				}
			}
		}()
	}

	// Fill our channel with the cases.
	for {
		c, err := e.cases.Next()
		if err == io.EOF {
			close(nextCases)
			break
		}
		nextCases <- nextCase[I, R]{c: c, iterErr: err}
	}

	// Wait for all the goroutines to finish.
	wg.Wait()
	elapsed := time.Since(start)

	err := errors.Join(errs.get()...)

	permalink := e.permalink()
	result := newResult(
		key{
			experimentID: e.experimentID,
			name:         e.experimentName,
			projectID:    e.projectID,
			projectName:  e.projectName,
		},
		err,
		permalink,
		elapsed,
	)

	// Print result summary unless quiet
	if !e.quiet {
		fmt.Println(result.String())
	}

	return result, err
}

// runNextCase handles a single case from the channel.
// Copied from old package.
func (e *eval[I, R]) runNextCase(ctx context.Context, nextCase nextCase[I, R]) error {
	// if we have a case or get an error, we'll create a span.
	ctx, span := e.tracer.Start(ctx, "eval", e.startSpanOpt)
	defer span.End()

	// if our case iterator returns an error, we'll wrap it in a more
	// specific error and short circuit.
	if nextCase.iterErr != nil {
		werr := fmt.Errorf("%w: %w", errCaseIterator, nextCase.iterErr)
		recordSpanError(span, werr)
		return werr
	}

	// otherwise let's run the case (using the existing span)
	return e.runCase(ctx, span, nextCase.c)
}

// runCase orchestrates task + scorers for one case.
// Copied from old package.
func (e *eval[I, R]) runCase(ctx context.Context, span oteltrace.Span, c Case[I, R]) error {
	if c.Tags != nil {
		span.SetAttributes(attribute.StringSlice("braintrust.tags", c.Tags))
	}

	result, err := e.runTask(ctx, c)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	_, err = e.runScorers(ctx, c, result)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	meta := map[string]any{
		"braintrust.span_attributes": evalSpanAttrs,
		"braintrust.input_json":      c.Input,
		"braintrust.output_json":     result,
		"braintrust.expected":        c.Expected,
	}

	// Add case metadata if present
	if c.Metadata != nil {
		meta["braintrust.metadata"] = c.Metadata
	}

	return setJSONAttrs(span, meta)
}

// runTask executes the task function and creates a task span.
// Copied from old package.
func (e *eval[I, R]) runTask(ctx context.Context, c Case[I, R]) (R, error) {
	ctx, span := e.tracer.Start(ctx, "task", e.startSpanOpt)
	defer span.End()

	attrs := map[string]any{
		"braintrust.input_json":      c.Input,
		"braintrust.expected":        c.Expected,
		"braintrust.span_attributes": taskSpanAttrs,
	}

	var encodeErrs []error
	for key, value := range attrs {
		if err := setJSONAttr(span, key, value); err != nil {
			encodeErrs = append(encodeErrs, err)
		}
	}

	result, err := e.task(ctx, c.Input)
	if err != nil {
		// if the task fails, don't worry about the encode errors....
		taskErr := fmt.Errorf("%w: %w", errTaskRun, err)
		recordSpanError(span, taskErr)
		return result, taskErr
	}

	if err := setJSONAttr(span, "braintrust.output_json", result); err != nil {
		encodeErrs = append(encodeErrs, err)
	}

	return result, errors.Join(encodeErrs...)
}

// runScorers executes all scorers and creates a score span.
// Copied from old package.
func (e *eval[I, R]) runScorers(ctx context.Context, c Case[I, R], result R) ([]Score, error) {
	ctx, span := e.tracer.Start(ctx, "score", e.startSpanOpt)
	defer span.End()

	if err := setJSONAttr(span, "braintrust.span_attributes", scoreSpanAttrs); err != nil {
		return nil, err
	}

	var scores []Score

	var errs []error
	for _, scorer := range e.scorers {
		curScores, err := scorer.Run(ctx, c.Input, c.Expected, result, c.Metadata)
		if err != nil {
			werr := fmt.Errorf("%w: scorer %q failed: %w", errScorer, scorer.Name(), err)
			recordSpanError(span, werr)
			errs = append(errs, werr)
			continue
		}
		for _, score := range curScores {
			if score.Name == "" {
				score.Name = scorer.Name()
			}
			scores = append(scores, score)
		}
	}

	// Build scores map (name -> score value)
	valsByName := make(map[string]float64, len(scores))
	for _, score := range scores {
		valsByName[score.Name] = score.Score
	}

	if err := setJSONAttr(span, "braintrust.scores", valsByName); err != nil {
		return nil, err
	}

	// Build metadata and output following Python/TypeScript conventions
	// Always build nested structure, then flatten if single score
	metadata := make(map[string]any, len(scores))
	output := make(map[string]any, len(scores))

	for _, score := range scores {
		if score.Metadata != nil {
			metadata[score.Name] = score.Metadata
		}
		output[score.Name] = map[string]any{"score": score.Score}
	}

	// For single score: flatten metadata and output to top level
	if len(scores) == 1 {
		score := scores[0]
		if score.Metadata != nil {
			if err := setJSONAttr(span, "braintrust.metadata", score.Metadata); err != nil {
				return nil, err
			}
		}
		if err := setJSONAttr(span, "braintrust.output", map[string]any{"score": score.Score}); err != nil {
			return nil, err
		}
	} else if len(scores) > 1 {
		// Multiple scores: use nested structure
		if len(metadata) > 0 {
			if err := setJSONAttr(span, "braintrust.metadata", metadata); err != nil {
				return nil, err
			}
		}
		if err := setJSONAttr(span, "braintrust.output", output); err != nil {
			return nil, err
		}
	}

	err := errors.Join(errs...) // will be nil if there are no errors
	return scores, err
}

// permalink generates a URL to view the eval in Braintrust UI.
// Copied from old package but adapted for injected dependencies.
func (e *eval[I, R]) permalink() string {
	appURL := e.config.AppURL
	orgName := e.config.OrgName

	// Try to get from session if login is complete
	if ok, info := e.session.Info(); ok {
		if appURL == "" && info.AppPublicURL != "" {
			appURL = info.AppPublicURL
		}
		if orgName == "" && info.OrgName != "" {
			orgName = info.OrgName
		}
	}

	if appURL == "" {
		appURL = "https://www.braintrust.dev"
	}

	if orgName != "" && e.experimentID != "" {
		return fmt.Sprintf("%s/app/%s/object?object_type=experiment&object_id=%s", appURL, orgName, e.experimentID)
	}

	return ""
}

// Run executes an evaluation using client resources (config, session, tracerProvider).
// This is the main entry point for eval execution.
func Run[I, R any](ctx context.Context, opts Opts[I, R], cfg *config.Config, session *auth.Session, tp *trace.TracerProvider) (*Result, error) {
	// Validate required fields
	if opts.Experiment == "" {
		return nil, fmt.Errorf("%w: Experiment is required", errEval)
	}
	if opts.Cases == nil {
		return nil, fmt.Errorf("%w: Cases is required", errEval)
	}
	if opts.Task == nil {
		return nil, fmt.Errorf("%w: Task is required", errEval)
	}

	// Create eval executor
	e, err := newEval(ctx, cfg, session, tp, opts)
	if err != nil {
		return nil, err
	}

	// Run evaluation
	return e.run(ctx)
}

// Helper functions (copied from old package)

func setJSONAttrs(span oteltrace.Span, attrs map[string]any) error {
	for key, value := range attrs {
		if err := setJSONAttr(span, key, value); err != nil {
			return err
		}
	}
	return nil
}

func setJSONAttr(span oteltrace.Span, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	span.SetAttributes(attribute.String(key, string(b)))
	return nil
}

func recordSpanError(span oteltrace.Span, err error) {
	// hardcode the error type when we know what it is. there may be better ways to do this
	// but by default otel would show *fmt.wrapErrors as the type, which isn't super nice to
	// look at. this function balances us returning errors which work with errors.Is() and
	// showing the actual error type in the braintrust ui.
	var errType string
	switch {
	case errors.Is(err, errScorer):
		errType = "ErrScorer"
	case errors.Is(err, errTaskRun):
		errType = "ErrTaskRun"
	case errors.Is(err, errCaseIterator):
		errType = "ErrCaseIterator"
	case errors.Is(err, errEval):
		errType = "ErrEval"
	default:
		errType = fmt.Sprintf("%T", err)
	}

	span.AddEvent("exception", oteltrace.WithAttributes(
		attribute.String("exception.type", errType),
		attribute.String("exception.message", err.Error()),
	))
	span.SetStatus(codes.Error, err.Error())
}

// lockedErrors is a thread-safe list of errors.
type lockedErrors struct {
	mu   sync.Mutex
	errs []error
}

func (e *lockedErrors) append(err error) {
	e.mu.Lock()
	e.errs = append(e.errs, err)
	e.mu.Unlock()
}

func (e *lockedErrors) get() []error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.errs
}

// minInt returns the minimum of two integers (Go 1.21+ has this in stdlib)
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// testNewEval creates an eval for unit testing, bypassing API calls.
// This allows tests to inject static values for experiment/project IDs.
func testNewEval[I, R any](
	cfg *config.Config,
	session *auth.Session,
	tracer oteltrace.Tracer,
	experimentID string,
	experimentName string,
	projectID string,
	projectName string,
	cases Cases[I, R],
	task Task[I, R],
	scorers []Scorer[I, R],
	parallelism int,
) *eval[I, R] {
	// Build parent span option
	parentAttr := fmt.Sprintf("experiment_id:%s", experimentID)
	startSpanOpt := oteltrace.WithAttributes(attribute.String("braintrust.parent", parentAttr))

	// Set parallelism
	goroutines := parallelism
	if goroutines < 1 {
		goroutines = 1
	}

	return &eval[I, R]{
		config:         cfg,
		session:        session,
		experimentID:   experimentID,
		experimentName: experimentName,
		projectID:      projectID,
		projectName:    projectName,
		cases:          cases,
		task:           task,
		scorers:        scorers,
		tracer:         tracer,
		startSpanOpt:   startSpanOpt,
		goroutines:     goroutines,
		quiet:          true,
	}
}
