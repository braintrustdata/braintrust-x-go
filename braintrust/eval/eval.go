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
//   - [Case][string, string] represents a test case with string input and string output
//   - [Task][MyInput, MyOutput] represents a task that takes MyInput and returns MyOutput
//   - [Cases][string, bool] represents an iterator over cases with string inputs and boolean outputs
//
// For complete usage examples, see the package examples.
package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/api"
	bttrace "github.com/braintrust/braintrust-x-go/braintrust/trace"
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

// Eval is a collection of cases, a task, and a set of scorers. It has two generic types;
// I is the input type, and R is the result type.
type Eval[I, R any] struct {
	experimentID string
	cases        Cases[I, R]
	task         Task[I, R]
	scorers      []Scorer[I, R]
	tracer       trace.Tracer
	parent       bttrace.Parent
	startSpanOpt trace.SpanStartOption
}

// New creates a new eval with the given experiment ID, cases, task, and scorers.
func New[I, R any](experimentID string, cases Cases[I, R], task Task[I, R], scorers []Scorer[I, R]) *Eval[I, R] {

	// Every span created from this eval will have the experiment ID as the parent. This _should_ be done by the SpanProcessor
	// but just in case a user hasn't set it up, we'll do it again here just in case as it should be idempotent.
	parent := bttrace.NewExperiment(experimentID)
	parentAttr := attr.String(bttrace.ParentOtelAttrKey, parent.String())
	startSpanOpt := trace.WithAttributes(parentAttr)

	return &Eval[I, R]{
		experimentID: experimentID,
		cases:        cases,
		task:         task,
		scorers:      scorers,
		startSpanOpt: startSpanOpt,
		parent:       parent,
		tracer:       otel.GetTracerProvider().Tracer("braintrust.eval"),
	}
}

// Run runs the eval.
func (e *Eval[I, R]) Run() error {
	if e.experimentID == "" {
		return fmt.Errorf("%w: experiment ID is required", ErrEval)
	}

	ctx := bttrace.SetParent(context.Background(), e.parent)

	var errs []error
	for {
		done, err := e.runNextCase(ctx)
		if done {
			break
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// runNextCase runs the next case in the iterator. It returns true if the iterator is
// exhausted and false otherwise, and any error that occurred while running the case.
func (e *Eval[I, R]) runNextCase(ctx context.Context) (done bool, err error) {
	c, err := e.cases.Next()
	done = err == io.EOF
	if done {
		return done, nil
	}

	// if we have a case or get an error, we'll create a span.
	ctx, span := e.tracer.Start(ctx, "eval", e.startSpanOpt)
	defer span.End()

	// if our case iterator returns an error, we'll wrap it in a more
	// specific error and short circuit.
	if err != nil {
		werr := fmt.Errorf("%w: %w", ErrCaseIterator, err)
		recordSpanError(span, werr)
		return done, werr
	}

	// otherwise let's run the case (using the existing span)
	return done, e.runCase(ctx, span, c)
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
// we will default to the score of the score.
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

// ResolveExperimentID resolves an experiment ID from a name and project ID.
func ResolveExperimentID(name string, projectID string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("experiment name is required")
	}
	if projectID == "" {
		return "", fmt.Errorf("project ID is required")
	}
	experiment, err := api.RegisterExperiment(name, projectID)
	if err != nil {
		return "", fmt.Errorf("failed to register experiment %q in project %q: %w", name, projectID, err)
	}
	return experiment.ID, nil
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
	return ResolveExperimentID(name, project.ID)
}

// QueryDataset queries a dataset from the Braintrust server and returns a Cases iterator that unmarshals
// the dataset JSON into the given input and expected types.
func QueryDataset[InputType, ExpectedType any](datasetID string) Cases[InputType, ExpectedType] {
	return &typedDatasetIterator[InputType, ExpectedType]{
		dataset: api.NewDataset(datasetID),
	}
}

type typedDatasetIterator[InputType, ExpectedType any] struct {
	dataset *api.Dataset
}

func (s *typedDatasetIterator[InputType, ExpectedType]) Next() (Case[InputType, ExpectedType], error) {
	var fullEvent struct {
		Input    InputType    `json:"input"`
		Expected ExpectedType `json:"expected"`
	}

	err := s.dataset.NextAs(&fullEvent)
	if err != nil {
		var zero Case[InputType, ExpectedType]
		return zero, err
	}

	return Case[InputType, ExpectedType]{
		Input:    fullEvent.Input,
		Expected: fullEvent.Expected,
	}, nil
}
