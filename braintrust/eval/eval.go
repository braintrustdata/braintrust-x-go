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
	scorers []*Scorer[I, R]
	tracer  trace.Tracer
}

func NewEval[I, R any](id string, cases []Case[I, R], task Task[I, R], scorers []*Scorer[I, R]) *Eval[I, R] {
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

		_, err = e.runScorers(ctx, c, result)
		if err != nil {
			span.End()
			return err
		}

		// save our span metadata.
		meta := map[string]any{
			"braintrust.span_attributes": map[string]any{
				"type": "eval",
			},
			"braintrust.input_json":  c.Input,
			"braintrust.output_json": result,
			"braintrust.expected":    c.Expected,
		}

		for key, value := range meta {
			if err := setJSONAttr(span, key, value); err != nil {
				return err
			}
		}

		span.End()
	}

	return nil
}

func (e *Eval[I, R]) runScorers(ctx context.Context, c Case[I, R], result R) ([]Score, error) {
	ctx, span := e.tracer.Start(ctx, "score")
	defer span.End()

	attrs := map[string]any{
		"type": "score",
	}

	if err := setJSONAttr(span, "braintrust.span_attributes", attrs); err != nil {
		return nil, err
	}

	scores := make([]Score, len(e.scorers))
	meta := make(map[string]float64, len(e.scorers))

	for i, scorer := range e.scorers {
		val, err := scorer.Run(ctx, c, result)
		if err != nil {
			span.RecordError(err)
			return scores, err
		}
		scores[i] = Score{
			Name:  scorer.Name,
			Score: val,
		}
		meta[scorer.Name] = val
	}

	if err := setJSONAttr(span, "braintrust.scores", meta); err != nil {
		return nil, err
	}

	return scores, nil
}

func runTask[I, R any](ctx context.Context, task Task[I, R], input I) (R, error) {
	tracer := tracer()

	ctx, span := tracer.Start(ctx, "task")
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

	meta := map[string]any{
		"type": "task",
	}

	if err := setJSONAttr(span, "braintrust.span_attributes", meta); err != nil {
		return result, err
	}

	return result, nil
}

// Task is a function that takes an input and returns a result.
type Task[I, R any] func(ctx context.Context, input I) (R, error)

// Scorers evaluate inputs / outputs and return a score between 0 and 1
type Scorer[I, R any] struct {
	Name string
	Run  func(ctx context.Context, c Case[I, R], result R) (float64, error)
}

func NewScorer[I, R any](name string, run func(ctx context.Context, c Case[I, R], result R) (float64, error)) *Scorer[I, R] {
	return &Scorer[I, R]{
		Name: name,
		Run:  run,
	}
}

type Case[I, R any] struct {
	Input    I
	Expected R
}

type Score struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
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
