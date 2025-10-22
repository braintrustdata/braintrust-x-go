// Package eval provides tools for evaluating AI model outputs.
// Evaluations help measure AI application performance (accuracy/quality) and create
// an effective feedback loop for AI development. They help teams understand if
// updates improve or regress application quality. Evaluations are a key part of
// the Braintrust platform.
//
// An evaluation consists of three main components:
//   - [Cases]: A set of test examples with inputs and expected outputs
//   - [Task]: The unit of work we are evaluating, usually one or more calls to an LLM
//   - [Scorer]: A function that scores the result of a task against the expected result
//
// # Type Parameters
//
// This package uses two generic type parameters throughout its API:
//   - I: The input type for the task (e.g., string, struct, []byte)
//   - R: The result/output type from the task (e.g., string, struct, complex types)
//
// All of the input and result types must be JSON-encodable.
//
// For example:
//   - [Case][string, string] is a test case with string input and string output
//   - [Task][Input, Output] is a task that takes Input and returns Output
//   - [Cases][string, bool] is an iterator of Cases with string inputs and boolean outputs
//
// See [Run] for the simplest way of running evals.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
	bttrace "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

var (
	// ErrEval is a generic error returned when an eval fails to execute.
	ErrEval = errors.New("eval error")

	// ErrScorer is returned when a scorer fails to execute.
	ErrScorer = errors.New("scorer error")

	// ErrTaskRun is returned when a task fails to execute.
	ErrTaskRun = errors.New("task run error")

	// ErrCaseIterator is returned when a case iterator fails to execute.
	ErrCaseIterator = errors.New("case iterator error")
)

var (
	// braintrust "span_attributes" for each type of eval span.
	evalSpanAttrs  = map[string]any{"type": "eval"}
	taskSpanAttrs  = map[string]any{"type": "task"}
	scoreSpanAttrs = map[string]any{"type": "score"}
)

// Key contains the data needed to uniquely identify and reference an eval.
type Key struct {
	ExperimentID string
	Name         string
	ProjectID    string
	ProjectName  string
}

// Eval is a collection of cases, a task, and a set of scorers. It has two generic types;
// I is the input type, and R is the result type. See [Run] for the simplest
// way of executing Evals.
type Eval[I, R any] struct {
	key          Key
	cases        Cases[I, R]
	task         Task[I, R]
	scorers      []Scorer[I, R]
	tracer       trace.Tracer
	parent       bttrace.Parent
	startSpanOpt trace.SpanStartOption
	goroutines   int
	quiet        bool
}

// New creates a new eval with the given experiment ID, cases, task, and scorers.
//
// [Run] is a preferable convenience function that automatically resolves the project, experiment, and cases, and adds options.
func New[I, R any](key Key, cases Cases[I, R], task Task[I, R], scorers []Scorer[I, R]) *Eval[I, R] {
	// Every span created from this eval will have the experiment ID as the parent. This _should_ be done by the SpanProcessor
	// but just in case a user hasn't set it up, we'll do it again here just in case as it should be idempotent.
	parent := bttrace.Parent{Type: bttrace.ParentTypeExperimentID, ID: key.ExperimentID}
	startSpanOpt := trace.WithAttributes(parent.Attr())

	return &Eval[I, R]{
		key:          key,
		cases:        cases,
		task:         task,
		scorers:      scorers,
		goroutines:   1,
		startSpanOpt: startSpanOpt,
		parent:       parent,
		tracer:       otel.GetTracerProvider().Tracer("braintrust.eval"),
	}
}

// setParallelism sets the number of goroutines used to run the eval.
func (e *Eval[I, R]) setParallelism(goroutines int) {
	if goroutines < 1 {
		log.Warnf("setParallelism: goroutines must be at least 1, defaulting to 1")
		goroutines = 1
	}
	e.goroutines = goroutines
}

// Permalink returns a URL to view this evaluation in the Braintrust UI.
func (e *Eval[I, R]) Permalink() (string, error) {
	config := braintrust.GetConfig()

	if e.key.ExperimentID == "" {
		return "", fmt.Errorf("experiment ID not set in eval key")
	}

	// Get app URL and org name - check config first, then cached auth state
	appURL := config.AppURL
	orgName := config.OrgName

	if (appURL == "" || orgName == "") && config.APIKey != "" {
		// Try to get from cached auth state
		if state, ok := auth.GetState(config.APIKey, config.OrgName); ok {
			if appURL == "" {
				appURL = state.AppPublicURL
			}
			if orgName == "" {
				orgName = state.OrgName
			}
		}
	}

	if appURL == "" {
		return "", fmt.Errorf("app URL not configured")
	}
	if orgName == "" {
		return "", fmt.Errorf("org name not available")
	}

	// Format: {app_url}/app/{org}/object?object_type=experiment&object_id={experiment_id}
	link := fmt.Sprintf("%s/app/%s/object?object_type=experiment&object_id=%s",
		appURL,
		orgName,
		e.key.ExperimentID,
	)

	return link, nil
}

// Run executes the evaluation by running the task on each case and scoring the results.
// Returns a [Result] and any error which contains all errors encountered during case iteration, task execution, and scoring.
func (e *Eval[I, R]) Run(ctx context.Context) (*Result, error) {
	start := time.Now()
	if e.key.ExperimentID == "" {
		return nil, fmt.Errorf("%w: experiment ID is required", ErrEval)
	}

	ctx = bttrace.SetParent(ctx, e.parent)

	// Scale buffer size with parallelism to avoid blocking, but cap at 100
	bufferSize := min(e.goroutines*2, 100)
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

	permalink, _ := e.Permalink() // err not super important here
	result := newResult(e.key, err, permalink, elapsed)
	if !e.quiet {
		fmt.Println(result.String())
	}

	return result, err
}

func (e *Eval[I, R]) runNextCase(ctx context.Context, nextCase nextCase[I, R]) error {
	// if we have a case or get an error, we'll create a span.
	ctx, span := e.tracer.Start(ctx, "eval", e.startSpanOpt)
	defer span.End()

	// if our case iterator returns an error, we'll wrap it in a more
	// specific error and short circuit.
	if nextCase.iterErr != nil {
		werr := fmt.Errorf("%w: %w", ErrCaseIterator, nextCase.iterErr)
		recordSpanError(span, werr)
		return werr
	}

	// otherwise let's run the case (using the existing span)
	return e.runCase(ctx, span, nextCase.c)
}

func (e *Eval[I, R]) runCase(ctx context.Context, span trace.Span, c Case[I, R]) error {
	if c.Tags != nil {
		span.SetAttributes(attr.StringSlice("braintrust.tags", c.Tags))
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

func (e *Eval[I, R]) runScorers(ctx context.Context, c Case[I, R], result R) (Scores, error) {
	ctx, span := e.tracer.Start(ctx, "score", e.startSpanOpt)
	defer span.End()

	if err := setJSONAttr(span, "braintrust.span_attributes", scoreSpanAttrs); err != nil {
		return nil, err
	}

	var scores Scores

	var errs []error
	for _, scorer := range e.scorers {
		curScores, err := scorer.Run(ctx, c.Input, c.Expected, result, c.Metadata)
		if err != nil {
			werr := fmt.Errorf("%w: scorer %q failed: %w", ErrScorer, scorer.Name(), err)
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

	valsByName := make(map[string]float64, len(scores))
	for _, score := range scores {
		valsByName[score.Name] = score.Score
	}

	if err := setJSONAttr(span, "braintrust.scores", valsByName); err != nil {
		return nil, err
	}

	err := errors.Join(errs...) // will be nil if there are no errors
	return scores, err
}

func (e *Eval[I, R]) runTask(ctx context.Context, c Case[I, R]) (R, error) {
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
		taskErr := fmt.Errorf("%w: %w", ErrTaskRun, err)
		recordSpanError(span, taskErr)
		return result, taskErr
	}

	if err := setJSONAttr(span, "braintrust.output_json", result); err != nil {
		encodeErrs = append(encodeErrs, err)
	}

	return result, errors.Join(encodeErrs...)
}

// Result contains the results from running an evaluation.
type Result struct {
	key       Key
	err       error
	elapsed   time.Duration
	permalink string
	// TODO: Will be populated with span data, scores, errors, etc. in future iterations
}

func newResult(key Key, err error, permalink string, elapsed time.Duration) *Result {
	return &Result{
		err:       err,
		permalink: permalink,
		elapsed:   elapsed,
		key:       key,
	}
}

// Permalink returns link to this eval in the Braintrust UI.
func (r *Result) Permalink() (string, error) {
	return r.permalink, nil
}

// Error returns the error from running the eval.
func (r *Result) Error() error {
	return r.err
}

// Name returns the experiment name.
func (r *Result) Name() string {
	return r.key.Name
}

// ID returns the experiment ID.
func (r *Result) ID() string {
	return r.key.ExperimentID
}

// String returns a string representaton of the result for printing on the console.
//
// The format it prints will change and shouldn't be relied on for programmatic use.
func (r *Result) String() string {
	link, linkErr := r.Permalink()

	projectDisplay := r.key.ProjectName
	if projectDisplay == "" {
		projectDisplay = r.key.ProjectID
	}

	lines := []string{
		"",
		fmt.Sprintf("=== Experiment: %s ===", r.key.Name),
		fmt.Sprintf("Name: %s", r.key.Name),
		fmt.Sprintf("Project: %s", projectDisplay),
		fmt.Sprintf("Duration: %.1fs", r.elapsed.Seconds()),
		fmt.Sprintf("Link: %s", link),
	}
	if linkErr != nil {
		log.Warnf("Failed to generate permalink: %v", linkErr)
	}

	// Error details if present
	if r.err != nil {
		lines = append(lines, "Errors:")
		lines = append(lines, "  "+r.err.Error())
	}

	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// Opts contains all options for running an evaluation in a single call.
type Opts[I, R any] struct {
	// Provide either Project name or Project ID
	Project   string
	ProjectID string

	// Required
	Task       Task[I, R]
	Scorers    []Scorer[I, R]
	Experiment string

	// Provide one of Cases, Dataset, or DatasetID
	Cases          Cases[I, R]
	Dataset        string
	DatasetID      string
	DatasetVersion string
	DatasetLimit   int // Max rows to fetch from dataset (0 = unlimited)

	// Options:
	Parallelism int                    // Number of goroutines (default: 1)
	Quiet       bool                   // Suppress result output (default: false)
	Tags        []string               // Tags to apply to the experiment
	Metadata    map[string]interface{} // Metadata to attach to the experiment
	Update      bool                   // If true, append to existing experiment instead of creating new one (default: false)
}

// Run executes an evaluation with automatic resolution of project, experiment, and dataset.
func Run[I, R any](ctx context.Context, opts Opts[I, R]) (*Result, error) {
	// Validate required fields (no API calls)
	if opts.Task == nil {
		return nil, fmt.Errorf("%w: Task is required", ErrEval)
	}
	if len(opts.Scorers) == 0 {
		return nil, fmt.Errorf("%w: at least one Scorer is required", ErrEval)
	}
	if opts.Experiment == "" {
		return nil, fmt.Errorf("%w: Experiment is required", ErrEval)
	}

	// Validate cases source before making API calls
	if err := validateCasesSource(opts); err != nil {
		return nil, err
	}

	// Attempt to login to cache org name for permalinks
	// Login() will use GetConfig() which returns the cached config from trace.Quickstart()
	// Ignore errors - permalinks will still work if org name is configured via env vars
	if _, err := braintrust.Login(); err != nil {
		log.Debugf("Could not login for permalink generation: %v", err)
	}

	// Resolve project ID (fall back to config defaults)
	projectID, err := resolveProjectID(opts.ProjectID, opts.Project, braintrust.GetConfig())
	if err != nil {
		return nil, err
	}

	// Resolve experiment ID and name with tags, metadata, and update flag
	experimentID, experimentName, err := resolveExpID(opts.Experiment, projectID, opts.Tags, opts.Metadata, opts.Update)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve experiment: %w", err)
	}

	// Resolve cases
	cases, err := resolveCases(opts, projectID)
	if err != nil {
		return nil, err
	}

	// Create and run the evaluation
	key := Key{ExperimentID: experimentID, Name: experimentName, ProjectID: projectID, ProjectName: opts.Project}

	eval := New(key, cases, opts.Task, opts.Scorers)
	if opts.Parallelism > 0 {
		eval.setParallelism(opts.Parallelism)
	}
	if opts.Quiet {
		eval.quiet = true
	}
	return eval.Run(ctx)
}

// Metadata is a map of strings to a JSON-encodable value. It is used to store arbitrary metadata about a case.
type Metadata map[string]any

// Task is a function that takes an input and returns a result. It represents the unit of work
// we are evaluating, usually one or more calls to an LLM.
type Task[I, R any] func(ctx context.Context, input I) (R, error)

// Case is the input and expected result of a test case.
type Case[I, R any] struct {
	Input    I
	Expected R
	Tags     []string
	Metadata Metadata
}

// Score represents the result of a scorer evaluation.
type Score struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// Scores is a list of scores.
type Scores []Score

// ScoreFunc is a function that evaluates a task (usually an LLM call) and returns a list of Scores.
type ScoreFunc[I, R any] func(ctx context.Context, input I, expected, result R, meta Metadata) (Scores, error)

// S is a helper function to concisely return a single score from ScoreFuncs. Scores created with S will default to the
// name of the scorer that creates them.
//
// `S(0.5)` is equivalent to `[]Score{{Score: 0.5}}`.
func S(score float64) Scores {
	return Scores{{Name: "", Score: score}}
}

// Scorer evaluates the quality of results against expected values. If a Scorer returns a score with an empty name,
// the score will default to the scorer's name.
type Scorer[I, R any] interface {
	Name() string
	Run(ctx context.Context, input I, expected, result R, meta Metadata) (Scores, error)
}

type scorerImpl[I, R any] struct {
	name      string
	scoreFunc ScoreFunc[I, R]
}

func (s *scorerImpl[I, R]) Name() string {
	return s.name
}

func (s *scorerImpl[I, R]) Run(ctx context.Context, input I, expected, result R, meta Metadata) (Scores, error) {
	return s.scoreFunc(ctx, input, expected, result, meta)
}

// NewScorer creates a new scorer with the given name and score function.
func NewScorer[I, R any](name string, scoreFunc ScoreFunc[I, R]) Scorer[I, R] {
	return &scorerImpl[I, R]{
		name:      name,
		scoreFunc: scoreFunc,
	}
}

func setJSONAttrs(span trace.Span, attrs map[string]any) error {
	for key, value := range attrs {
		if err := setJSONAttr(span, key, value); err != nil {
			return err
		}
	}
	return nil
}

func setJSONAttr(span trace.Span, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	span.SetAttributes(attr.String(key, string(b)))
	return nil
}

func recordSpanError(span trace.Span, err error) {
	// hardcode the error type when we know what it is. there may be better ways to do this
	// but by default otel would show *fmt.wrapErrors as the type, which isn't super nice to
	// look at. this function balances us returning errors which work with errors.Is() and
	// showing the actual error type in the braintrust ui.
	var errType string
	switch {
	case errors.Is(err, ErrScorer):
		errType = "ErrScorer"
	case errors.Is(err, ErrTaskRun):
		errType = "ErrTaskRun"
	case errors.Is(err, ErrCaseIterator):
		errType = "ErrCaseIterator"
	case errors.Is(err, ErrEval):
		errType = "ErrEval"
	default:
		errType = fmt.Sprintf("%T", err)
	}

	span.AddEvent("exception", trace.WithAttributes(
		attr.String("exception.type", errType),
		attr.String("exception.message", err.Error()),
	))
	span.SetStatus(codes.Error, err.Error())
}

// Cases is an iterator of test cases that are evaluated by [Eval]. Implementations must return
// io.EOF when iteration is complete.
//
// See [QueryDataset] to download datasets or [NewCases] to easily wrap slices of cases.
type Cases[I, R any] interface {
	// Next must return the next case in the dataset, or io.EOF if there are no more cases.
	// The returned case must be a valid input for the task.
	Next() (Case[I, R], error)
}

// NewCases creates a Cases iterator from a slice of cases.
func NewCases[I, R any](cases []Case[I, R]) Cases[I, R] {
	return &casesImpl[I, R]{
		cases: cases,
		index: 0,
	}
}

type casesImpl[I, R any] struct {
	cases []Case[I, R]
	index int
}

func (s *casesImpl[I, R]) Next() (Case[I, R], error) {
	if s.index >= len(s.cases) {
		var zero Case[I, R]
		return zero, io.EOF
	}
	testCase := s.cases[s.index]
	s.index++
	return testCase, nil
}

// ResolveKey creates a Key by resolving a project name and experiment name to their IDs.
func ResolveKey(projectName, experimentName string) (Key, error) {
	projectID, err := resolveProjectID("", projectName, braintrust.GetConfig())
	if err != nil {
		return Key{}, fmt.Errorf("failed to resolve project: %w", err)
	}
	experimentID, resolvedExpName, err := ResolveExperimentID(experimentName, projectID)
	if err != nil {
		return Key{}, fmt.Errorf("failed to resolve experiment: %w", err)
	}
	return Key{ExperimentID: experimentID, Name: resolvedExpName, ProjectID: projectID, ProjectName: projectName}, nil
}

// ResolveExperimentID resolves an experiment ID and name from a name and project ID.
// Returns (experimentID, experimentName, error).
func ResolveExperimentID(name string, projectID string) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("experiment name is required")
	}
	if projectID == "" {
		return "", "", fmt.Errorf("project ID is required")
	}
	experiment, err := api.RegisterExperiment(name, projectID, api.RegisterExperimentOpts{})
	if err != nil {
		return "", "", fmt.Errorf("failed to register experiment %q in project %q: %w", name, projectID, err)
	}
	return experiment.ID, experiment.Name, nil
}

// resolveExpID is an internal helper that resolves an experiment ID with tags, metadata, and update flag.
// Returns (experimentID, experimentName, error).
func resolveExpID(name string, projectID string, tags []string, metadata map[string]interface{}, update bool) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("experiment name is required")
	}
	if projectID == "" {
		return "", "", fmt.Errorf("project ID is required")
	}
	experiment, err := api.RegisterExperiment(name, projectID, api.RegisterExperimentOpts{
		Tags:     tags,
		Metadata: metadata,
		Update:   update,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to register experiment %q in project %q: %w", name, projectID, err)
	}
	return experiment.ID, experiment.Name, nil
}

// ResolveProjectExperimentID resolves an experiment ID from a name and project name.
func ResolveProjectExperimentID(name string, projectName string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("experiment name is required")
	}
	if projectName == "" {
		return "", fmt.Errorf("project name is required")
	}
	project, err := api.RegisterProject(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to register project %q: %w", projectName, err)
	}
	expID, _, err := ResolveExperimentID(name, project.ID)
	return expID, err
}

// resolveProjectID resolves a project ID from the provided options and config.
// It checks in order: explicit ProjectID, Project name, default ProjectID from config,
// default Project name from config. Returns an error if none are provided.
func resolveProjectID(projectID, projectName string, config braintrust.Config) (string, error) {
	if projectID != "" {
		return projectID, nil
	}

	if projectName != "" {
		project, err := api.RegisterProject(projectName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve project %q: %w", projectName, err)
		}
		return project.ID, nil
	}

	if config.DefaultProjectID != "" {
		return config.DefaultProjectID, nil
	}

	if config.DefaultProjectName != "" {
		project, err := api.RegisterProject(config.DefaultProjectName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve default project %q: %w", config.DefaultProjectName, err)
		}
		return project.ID, nil
	}

	return "", fmt.Errorf("%w: Project or ProjectID is required (or set BRAINTRUST_DEFAULT_PROJECT_ID)", ErrEval)
}

// validateCasesSource validates that exactly one case source is provided.
func validateCasesSource[I, R any](opts Opts[I, R]) error {
	paramCount := 0
	if opts.Cases != nil {
		paramCount++
	}
	if opts.Dataset != "" {
		paramCount++
	}
	if opts.DatasetID != "" {
		paramCount++
	}

	if paramCount == 0 {
		return fmt.Errorf("%w: one of Cases, Dataset, or DatasetID is required", ErrEval)
	}
	if paramCount > 1 {
		return fmt.Errorf("%w: only one of Cases, Dataset, or DatasetID should be provided", ErrEval)
	}
	return nil
}

// resolveCases resolves cases from the provided options.
// Exactly one of Cases, Dataset, or DatasetID must be provided (validated by validateCasesSource).
func resolveCases[I, R any](opts Opts[I, R], projectID string) (Cases[I, R], error) {

	// Resolve cases
	if opts.Cases != nil {
		return opts.Cases, nil
	}

	datasetOpts := DatasetOpts{
		ProjectID:   projectID,
		DatasetID:   opts.DatasetID,
		DatasetName: opts.Dataset,
		Version:     opts.DatasetVersion,
		Limit:       opts.DatasetLimit,
	}

	cases, err := QueryDataset[I, R](datasetOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset: %w", err)
	}
	return cases, nil
}

// nextCase represents the result of a call to Cases.Next(). It can contain a
// case or a legitimate error from iterator (e.g. an error paginating the dataset).
type nextCase[I, R any] struct {
	c       Case[I, R]
	iterErr error
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
