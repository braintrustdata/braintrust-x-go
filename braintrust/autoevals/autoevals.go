// Package autoevals provides scoring functions for evaluating AI model outputs.
//
// This package includes built-in scorers for common evaluation tasks and supports
// creating custom scorers for specific use cases.
//
// Example usage:
//
//	equals := autoevals.NewEquals[string, string]()
//	score, err := equals.Run(ctx, "input", "expected", "actual")
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Score: %.2f\n", score) // Score: 0.00 (since "expected" != "actual")
package autoevals

import (
	"context"

	"golang.org/x/exp/constraints"

	"github.com/braintrust/braintrust-x-go/braintrust/eval"
)

// Scorer evaluates the quality of results against expected values.
type Scorer[I, R any] interface {
	Name() string
	Run(ctx context.Context, input I, expected, result R, meta eval.Metadata) (eval.Scores, error)
}

type scorer[I, R any] struct {
	name      string
	scoreFunc eval.ScoreFunc[I, R]
}

func (s *scorer[I, R]) Name() string {
	return s.name
}

func (s *scorer[I, R]) Run(ctx context.Context, input I, expected, result R, meta eval.Metadata) (eval.Scores, error) {
	return s.scoreFunc(ctx, input, expected, result, meta)
}

// NewScorer creates a new scorer with the given name and score function.
func NewScorer[I, R any](name string, scoreFunc eval.ScoreFunc[I, R]) Scorer[I, R] {
	return &scorer[I, R]{
		name:      name,
		scoreFunc: scoreFunc,
	}
}

// NewEquals creates a scorer that returns 1.0 when result equals expected, 0.0 otherwise.
//
// Example:
//
//	equals := autoevals.NewEquals[string, string]()
//	score, err := equals.Run(ctx, "input", "hello", "hello") // returns 1.0
func NewEquals[I any, R comparable]() Scorer[I, R] {
	return NewScorer("Equals", func(_ context.Context, _ I, expected, result R, _ eval.Metadata) (eval.Scores, error) {
		v := 0.0
		if expected == result {
			v = 1.0
		}
		return eval.Scores{{Name: "Equals", Score: v}}, nil
	})
}

// NewLessThan creates a scorer that returns 1.0 when expected < result, 0.0 otherwise.
//
// Example:
//
//	lessThan := autoevals.NewLessThan[string, float64]()
//	score, err := lessThan.Run(ctx, "input", 0.5, 0.8) // returns 1.0 (0.5 < 0.8)
func NewLessThan[I any, R constraints.Ordered]() Scorer[I, R] {
	return NewScorer("LessThan", func(_ context.Context, _ I, expected, result R, _ eval.Metadata) (eval.Scores, error) {
		v := 0.0
		if expected < result {
			v = 1.0
		}
		return eval.Scores{{Name: "LessThan", Score: v}}, nil
	})
}
