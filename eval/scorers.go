package eval

import (
	"context"
)

// Scores is a list of scores.
type Scores []Score

// S is a helper function to concisely return a single score from scorers.
// Scores created with S will default to the name of the scorer that creates them.
// S(0.5) is equivalent to []Score{{Score: 0.5}}.
func S(score float64) Scores {
	return Scores{{Name: "", Score: score}}
}

// ScoreFunc is a function that evaluates a task and returns a list of Scores.
type ScoreFunc[I, R any] func(ctx context.Context, input I, expected, result R, meta Metadata) (Scores, error)

// NewScorer creates a new scorer with the given name and score function.
func NewScorer[I, R any](name string, scoreFunc ScoreFunc[I, R]) Scorer[I, R] {
	return &scorerImpl[I, R]{
		name:      name,
		scoreFunc: scoreFunc,
	}
}

type scorerImpl[I, R any] struct {
	name      string
	scoreFunc ScoreFunc[I, R]
}

func (s *scorerImpl[I, R]) Name() string {
	return s.name
}

func (s *scorerImpl[I, R]) Run(ctx context.Context, input I, expected, result R, meta Metadata) ([]Score, error) {
	return s.scoreFunc(ctx, input, expected, result, meta)
}
