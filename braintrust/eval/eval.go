package eval

import (
	"context"
	"encoding/json"

	bttrace "github.com/braintrust/braintrust-x-go/braintrust/trace"
	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Eval[I, R any] struct {
	id      string
	cases   []Case[I, R]
	task    Task[I, R]
	scorers []Scorer[I, R]
	tracer  trace.Tracer
}

func NewEval[I, R any](id string, cases []Case[I, R], task Task[I, R], scorers []Scorer[I, R]) *Eval[I, R] {
	return &Eval[I, R]{
		id:      id,
		cases:   cases,
		task:    task,
		scorers: scorers,
		tracer:  otel.GetTracerProvider().Tracer("braintrust.evals"),
	}
}

func (e *Eval[I, R]) Run() error {
	parent := bttrace.NewExperiment(e.id)

	for _, c := range e.cases {
		ctx := bttrace.SetParent(context.Background(), parent)
		ctx, span := e.tracer.Start(ctx, "eval")

		result, err := runTask(ctx, e.task, c.Input)
		if err != nil {
			span.End()
			return err
		}

		for _, scorer := range e.scorers {
			err := e.score(ctx, scorer, c, result)
			if err != nil {
				span.End()
				return err
			}
		}
		span.End()
	}

	return nil
}

func (e *Eval[I, R]) score(ctx context.Context, scorer Scorer[I, R], c Case[I, R], result R) error {
	ctx, span := e.tracer.Start(ctx, "score",
		trace.WithAttributes(attr.String("type", "eval.score")),
	)
	defer span.End()

	score, err := scorer(ctx, c.Input, result)
	if err != nil {
		span.RecordError(err)
		return err
	}

	input := map[string]any{
		"input":    c.Input,
		"expected": c.Expected,
		"output":   result,
	}

	if err := setJSONAttr(span, "braintrust.input", input); err != nil {
		return err
	}

	output := map[string]any{
		"score": score,
	}
	if err := setJSONAttr(span, "braintrust.output", output); err != nil {
		return err
	}

	return nil
}

func runTask[I, R any](ctx context.Context, task Task[I, R], input I) (R, error) {
	tracer := tracer()

	ctx, span := tracer.Start(ctx, "task",
		trace.WithAttributes(attr.String("type", "eval.task")),
	)
	defer span.End()

	result, err := task(ctx, input)
	if err != nil {
		span.RecordError(err)
		return result, err
	}

	if err := setJSONAttr(span, "braintrust.input", input); err != nil {
		return result, err
	}

	if err := setJSONAttr(span, "braintrust.output", result); err != nil {
		return result, err
	}

	metadata := map[string]any{
		"type": "task",
	}

	if err := setJSONAttr(span, "braintrust.metadata", metadata); err != nil {
		return result, err
	}

	return result, nil
}

// Task is a function that takes an input and returns a result.
type Task[I, R any] func(ctx context.Context, input I) (R, error)

// Scorer is a function that takes an input, result, and expected result and returns a score between 0 and 1
type Scorer[I, R any] func(ctx context.Context, input I, result R) (float64, error)

type Case[I, R any] struct {
	Input    I
	Expected R
}

func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust.evals")
}

func setJSONAttr(span trace.Span, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	span.SetAttributes(attr.String(key, string(b)))
	return nil
}
