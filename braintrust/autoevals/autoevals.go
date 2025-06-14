package autoevals

import (
	"context"

	"golang.org/x/exp/constraints"
)

// ScoreFunc is a function that scores the result of a task against the expected result.
type ScoreFunc[I, R any] func(ctx context.Context, input I, expected, result R) (float64, error)

type Scorer[I, R any] struct {
	name      string
	scoreFunc ScoreFunc[I, R]
}

func (s *Scorer[I, R]) Name() string {
	return s.name
}

func (s *Scorer[I, R]) Run(ctx context.Context, input I, expected, result R) (float64, error) {
	return s.scoreFunc(ctx, input, expected, result)
}

// NewScorer creates a new scorer with the given name and score function.
func NewScorer[I, R any](name string, scoreFunc ScoreFunc[I, R]) *Scorer[I, R] {
	return &Scorer[I, R]{
		name:      name,
		scoreFunc: scoreFunc,
	}
}

func NewEquals[I any, R comparable]() *Scorer[I, R] {
	return NewScorer("Equals", func(ctx context.Context, input I, expected, result R) (float64, error) {
		if expected == result {
			return 1, nil
		}
		return 0, nil
	})
}

func NewLessThan[I any, R constraints.Ordered]() *Scorer[I, R] {
	return NewScorer("LessThan", func(ctx context.Context, input I, expected, result R) (float64, error) {
		if expected < result {
			return 1, nil
		}
		return 0, nil
	})
}
