// Package eval provides functionality for running evaluations of AI model outputs.
// Evaluations help measure AI application performance (accuracy/quality) and create
// an effective feedback loop for AI development. They help teams understand if
// updates improve or regress application quality.
//
// An evaluation consists of three main components:
//   - Data: A set of test examples with inputs and expected outputs
//   - Task: An AI function that takes an input and returns an output
//   - Scores: Scoring functions that compute performance metrics
//
// Example usage:
//
//	import (
//		"context"
//		"log"
//		"github.com/braintrust/braintrust-x-go/braintrust/eval"
//		"github.com/braintrust/braintrust-x-go/braintrust/trace"
//	)
//
//	// Set up tracing (requires BRAINTRUST_API_KEY)
//	// export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
//	// This task is hardcoded but usually you'd call an AI model here.
//	greetingTask := func(ctx context.Context, input string) (string, error) {
//		return "Hello " + input, nil
//	}
//
//	// Define your scoring function
//	exactMatch := func(ctx context.Context, input, expected, result string) (float64, error) {
//		if expected == result {
//			return 1.0, nil // Perfect match
//		}
//		return 0.0, nil // No match
//	}
//
//	// Create and run the evaluation
//	evaluation, err := eval.NewWithOpts(
//		eval.Options{
//			ProjectName:    "my-ai-project",
//			ExperimentName: "greeting-experiment-v1",
//		},
//		[]eval.Case[string, string]{
//			{Input: "World", Expected: "Hello World"},
//			{Input: "Alice", Expected: "Hello Alice"},
//			{Input: "Bob", Expected: "Hello Bob"},
//		},
//		greetingTask,
//		[]eval.Scorer[string, string]{
//			eval.NewScorer("exact_match", exactMatch),
//		},
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	summary, err := evaluation.Run(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Evaluation completed. Average score: %.2f\n", summary.Scores["exact_match"])
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/api"
	bttrace "github.com/braintrust/braintrust-x-go/braintrust/trace"
)

var (
	// ErrScorer is returned when a scorer fails to execute.
	ErrScorer = errors.New("scorer error")

	// ErrTaskRun is returned when a task fails to execute.
	ErrTaskRun = errors.New("task run error")
)

var (
	// braintrust "span_attributes" for each type of eval span.
	evalSpanAttrs  = map[string]any{"type": "eval"}
	taskSpanAttrs  = map[string]any{"type": "task"}
	scoreSpanAttrs = map[string]any{"type": "score"}
)

// Options holds configuration for creating an eval
type Options struct {
	ProjectName    string
	ProjectID      string
	ExperimentName string
}

// NewWithOpts creates a new eval using options to resolve project and experiment.
// It can handle:
// - ProjectName + ExperimentName: creates/gets project, then creates/gets experiment
// - ProjectID + ExperimentName: uses existing project, creates/gets experiment
func NewWithOpts[I, R any](opts Options, cases []Case[I, R], task Task[I, R], scorers []Scorer[I, R]) (*Eval[I, R], error) {
	var projectID string
	var err error

	// Resolve project ID
	if opts.ProjectID != "" {
		projectID = opts.ProjectID
	} else if opts.ProjectName != "" {
		project, err := api.RegisterProject(opts.ProjectName)
		if err != nil {
			return nil, fmt.Errorf("failed to register project %q: %w", opts.ProjectName, err)
		}
		projectID = project.ID
	} else {
		return nil, fmt.Errorf("must provide either ProjectName or ProjectID")
	}

	// Resolve experiment ID
	if opts.ExperimentName == "" {
		return nil, fmt.Errorf("ExperimentName is required")
	}

	experiment, err := api.RegisterExperiment(opts.ExperimentName, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to register experiment %q: %w", opts.ExperimentName, err)
	}

	return New(experiment.ID, cases, task, scorers), nil
}

// Eval is a collection of cases, a task, and a set of scorers.
type Eval[I, R any] struct {
	id      string
	cases   []Case[I, R]
	task    Task[I, R]
	scorers []Scorer[I, R]
	tracer  trace.Tracer
}

// New creates a new eval.
func New[I, R any](id string, cases []Case[I, R], task Task[I, R], scorers []Scorer[I, R]) *Eval[I, R] {
	return &Eval[I, R]{
		id:      id,
		cases:   cases,
		task:    task,
		scorers: scorers,
		tracer:  otel.GetTracerProvider().Tracer("braintrust.eval"),
	}
}

// Run runs the eval.
func (e *Eval[I, R]) Run() error {
	parent := bttrace.NewExperiment(e.id)
	ctx := bttrace.SetParent(context.Background(), parent)

	var errs []error
	for _, c := range e.cases {
		err := e.runCase(ctx, c)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (e *Eval[I, R]) runCase(ctx context.Context, c Case[I, R]) error {
	ctx, span := e.tracer.Start(ctx, "eval")
	defer span.End()

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

	return setJSONAttrs(span, meta)
}

func (e *Eval[I, R]) runScorers(ctx context.Context, c Case[I, R], result R) ([]Score, error) {
	ctx, span := e.tracer.Start(ctx, "score")
	defer span.End()

	if err := setJSONAttr(span, "braintrust.span_attributes", scoreSpanAttrs); err != nil {
		return nil, err
	}

	scores := make([]Score, len(e.scorers))
	meta := make(map[string]float64, len(e.scorers))

	var errs []error

	for i, scorer := range e.scorers {
		val, err := scorer.Run(ctx, c.Input, c.Expected, result)
		if err != nil {
			werr := fmt.Errorf("%w: scorer %q failed: %w", ErrScorer, scorer.Name(), err)
			span.RecordError(werr)
			errs = append(errs, werr)
			continue
		}

		scores[i] = Score{Name: scorer.Name(), Score: val}
		meta[scorer.Name()] = val
	}

	if err := setJSONAttr(span, "braintrust.scores", meta); err != nil {
		return nil, err
	}

	err := errors.Join(errs...)
	if err != nil {
		return scores, fmt.Errorf("%w: %w", ErrScorer, err)
	}

	return scores, err
}

func (e *Eval[I, R]) runTask(ctx context.Context, c Case[I, R]) (R, error) {
	ctx, span := e.tracer.Start(ctx, "task")
	defer span.End()

	result, err := e.task(ctx, c.Input)
	if err != nil {
		taskErr := fmt.Errorf("%w: %w", ErrTaskRun, err)
		span.RecordError(taskErr)
		span.SetStatus(codes.Error, taskErr.Error())
		return result, taskErr
	}

	attrs := map[string]any{
		"braintrust.input_json":      c.Input,
		"braintrust.output_json":     result,
		"braintrust.expected":        c.Expected,
		"braintrust.span_attributes": taskSpanAttrs,
	}

	var errs []error
	for key, value := range attrs {
		if err := setJSONAttr(span, key, value); err != nil {
			errs = append(errs, err)
		}
	}

	return result, errors.Join(errs...)
}

// Task is a function that takes an input and returns a result. It represents the units of work
// that eval is trying to evaluate, like a model, a prompt, whatever.
type Task[I, R any] func(ctx context.Context, input I) (R, error)

// Case is the input and expected result of a test case.
type Case[I, R any] struct {
	Input    I
	Expected R
}

// Score represents the result of a scorer evaluation.
type Score struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// ScoreFunc is a function that scores the result of a task against the expected result.
type ScoreFunc[I, R any] func(ctx context.Context, input I, expected, result R) (float64, error)

// Scorer evaluates the quality of results against expected values.
type Scorer[I, R any] interface {
	Name() string
	Run(ctx context.Context, input I, expected, result R) (float64, error)
}

type scorerImpl[I, R any] struct {
	name      string
	scoreFunc ScoreFunc[I, R]
}

func (s *scorerImpl[I, R]) Name() string {
	return s.name
}

func (s *scorerImpl[I, R]) Run(ctx context.Context, input I, expected, result R) (float64, error) {
	return s.scoreFunc(ctx, input, expected, result)
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
