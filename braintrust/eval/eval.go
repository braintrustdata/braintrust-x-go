package eval

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const experimentIDKey = contextKey("braintrust-experiment-id")

type Eval[I, R any] struct {
	Cases   []Case[I, R]
	Task    Task[I, R]
	Scorers []Scorer[I, R]
}

func NewEval[I, R any](cases []Case[I, R], task Task[I, R], scorers []Scorer[I, R]) *Eval[I, R] {
	return &Eval[I, R]{
		Cases:   cases,
		Task:    task,
		Scorers: scorers,
	}
}

func (e *Eval[I, R]) Run() error {
	ctx := context.Background()

	ctx = context.WithValue(ctx, experimentIDKey, "ef909c25-8978-46ef-88f3-c42c4d1f3150")

	results := make([]R, len(e.Cases))

	for i, c := range e.Cases {
		result, err := runTask(ctx, e.Task, c.Input)
		if err != nil {
			return err
		}

		results[i] = result
	}

	return nil
}

func runTask[I, R any](ctx context.Context, task Task[I, R], input I) (R, error) {
	tracer := tracer()

	ctx, span := tracer.Start(ctx, "eval",
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
