// Package autoevals provides scoring functions for evaluating AI model outputs.
package autoevals

import (
	"context"

	"golang.org/x/exp/constraints"
)

// ScoreFunc is a function that scores the result of a task against the expected result.
type ScoreFunc[I, R any] func(ctx context.Context, input I, expected, result R) (float64, error)

// Scorer evaluates the quality of results against expected values.
type Scorer[I, R any] interface {
	Name() string
	Run(ctx context.Context, input I, expected, result R) (float64, error)
}

type scorer[I, R any] struct {
	name      string
	scoreFunc ScoreFunc[I, R]
}

func (s *scorer[I, R]) Name() string {
	return s.name
}

func (s *scorer[I, R]) Run(ctx context.Context, input I, expected, result R) (float64, error) {
	return s.scoreFunc(ctx, input, expected, result)
}

// NewScorer creates a new scorer with the given name and score function.
func NewScorer[I, R any](name string, scoreFunc ScoreFunc[I, R]) Scorer[I, R] {
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
	return NewScorer("Equals", func(_ context.Context, _ I, expected, result R) (float64, error) {
		if expected == result {
			return 1, nil
		}
		return 0, nil
	})
}

// NewLessThan creates a scorer that returns 1.0 when expected < result, 0.0 otherwise.
//
// Example:
//
//	lessThan := autoevals.NewLessThan[string, float64]()
//	score, err := lessThan.Run(ctx, "input", 0.5, 0.8) // returns 1.0 (0.5 < 0.8)
func NewLessThan[I any, R constraints.Ordered]() Scorer[I, R] {
	return NewScorer("LessThan", func(_ context.Context, _ I, expected, result R) (float64, error) {
		if expected < result {
			return 1, nil
		}
		return 0, nil
	})
}
