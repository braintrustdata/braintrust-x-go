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
	"fmt"

	"golang.org/x/exp/constraints"

	"github.com/braintrust/braintrust-x-go/braintrust/api"
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

// NewLevenshtein creates a scorer that calculates the normalized Levenshtein distance
// between expected and result strings using the Braintrust global Levenshtein function.
// Returns a score from 0.0 (completely different) to 1.0 (identical).
//
// Example:
//
//	levenshtein := autoevals.NewLevenshtein[string]()
//	score, err := levenshtein.Run(ctx, "input", "hello", "helo") // returns ~0.8 (high similarity)
func NewLevenshtein[I any]() Scorer[I, string] {
	return NewScorer("Levenshtein", func(ctx context.Context, _ I, expected, result string, _ eval.Metadata) (eval.Scores, error) {
		// Prepare request for the global Levenshtein function
		request := api.InvokeFunctionRequest{
			Input: map[string]interface{}{
				"expected": expected,
				"result":   result,
			},
		}

		// Call the global Levenshtein function
		response, err := api.InvokeGlobalFunction("Levenshtein", request)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke Levenshtein function: %w", err)
		}

		if response.Error != nil {
			return nil, fmt.Errorf("levenshtein function returned error: %s", *response.Error)
		}

		// Extract the score from the response
		var score float64
		if output, ok := response.Output.(map[string]interface{}); ok {
			if scoreVal, exists := output["score"]; exists {
				if scoreFloat, ok := scoreVal.(float64); ok {
					score = scoreFloat
				} else {
					return nil, fmt.Errorf("levenshtein function returned non-numeric score: %v", scoreVal)
				}
			} else {
				return nil, fmt.Errorf("levenshtein function output missing 'score' field")
			}
		} else {
			// Try direct output as float64
			if scoreFloat, ok := response.Output.(float64); ok {
				score = scoreFloat
			} else {
				return nil, fmt.Errorf("levenshtein function returned unexpected output format: %v", response.Output)
			}
		}

		return eval.Scores{{Name: "Levenshtein", Score: score}}, nil
	})
}
