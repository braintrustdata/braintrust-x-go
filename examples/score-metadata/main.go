package main

import (
	"context"
	"log"
	"strings"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	// Set up tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	log.Println("🚀 Running evaluation with score metadata...")

	// Run evaluation with scorers that include metadata
	_, err = eval.Run(context.Background(), eval.Opts[string, string]{
		// Project:    "go-sdk-examples",
		Project:    "pedro-project1",
		Experiment: "score-metadata-example",
		Cases: eval.NewCases([]eval.Case[string, string]{
			{
				Input:    "hello world",
				Expected: "HELLO WORLD",
				Metadata: map[string]interface{}{"case_id": "1", "category": "greeting"},
			},
			{
				Input:    "braintrust",
				Expected: "BRAINTRUST",
				Metadata: map[string]interface{}{"case_id": "2", "category": "product"},
			},
			{
				Input:    "go sdk",
				Expected: "GO SDK",
				Metadata: map[string]interface{}{"case_id": "3", "category": "tech"},
			},
		}),
		Task: func(ctx context.Context, input string) (string, error) {
			// Simple task: convert to uppercase
			return strings.ToUpper(input), nil
		},
		Scorers: []eval.Scorer[string, string]{
			// Scorer 1: Exact match with debug metadata
			eval.NewScorer("exact_match", func(ctx context.Context, input, expected, result string, caseMeta eval.Metadata) (eval.Scores, error) {
				matches := expected == result
				score := 0.0
				if matches {
					score = 1.0
				}

				return eval.Scores{
					{
						Name:  "exact_match",
						Score: score,
						Metadata: map[string]any{
							"reasoning":      "Checked if result exactly matches expected output",
							"input_length":   len(input),
							"expected":       expected,
							"result":         result,
							"case_metadata":  caseMeta,
							"match_status":   matches,
						},
					},
				}, nil
			}),

			// Scorer 2: Length similarity with detailed analysis
			eval.NewScorer("length_similarity", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
				expectedLen := len(expected)
				resultLen := len(result)

				// Calculate similarity: 1.0 if lengths match, otherwise ratio
				var similarity float64
				if expectedLen == resultLen {
					similarity = 1.0
				} else if expectedLen > 0 {
					similarity = float64(resultLen) / float64(expectedLen)
					if similarity > 1.0 {
						similarity = 1.0 / similarity
					}
				}

				return eval.Scores{
					{
						Name:  "length_similarity",
						Score: similarity,
						Metadata: map[string]any{
							"expected_length": expectedLen,
							"result_length":   resultLen,
							"length_diff":     resultLen - expectedLen,
							"debug": map[string]any{
								"calculation": "min(result_len, expected_len) / max(result_len, expected_len)",
								"similarity":  similarity,
							},
						},
					},
				}, nil
			}),

			// Scorer 3: Character-by-character analysis
			eval.NewScorer("character_analysis", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
				matchCount := 0
				minLen := len(expected)
				if len(result) < minLen {
					minLen = len(result)
				}

				for i := 0; i < minLen; i++ {
					if expected[i] == result[i] {
						matchCount++
					}
				}

				var charMatchScore float64
				if len(expected) > 0 {
					charMatchScore = float64(matchCount) / float64(len(expected))
				}

				return eval.Scores{
					{
						Name:  "character_match",
						Score: charMatchScore,
						Metadata: map[string]any{
							"matched_chars":    matchCount,
							"total_chars":      len(expected),
							"match_percentage": charMatchScore * 100,
							"analysis": map[string]any{
								"expected_length": len(expected),
								"result_length":   len(result),
								"first_mismatch":  findFirstMismatch(expected, result),
							},
						},
					},
				}, nil
			}),
		},
	})

	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	log.Println("✅ Evaluation completed successfully!")
	log.Println("📊 Check the Braintrust dashboard to see score metadata in the experiment details")
}

// Helper function to find the first character that doesn't match
func findFirstMismatch(expected, result string) map[string]any {
	minLen := len(expected)
	if len(result) < minLen {
		minLen = len(result)
	}

	for i := 0; i < minLen; i++ {
		if expected[i] != result[i] {
			return map[string]any{
				"position":        i,
				"expected_char":   string(expected[i]),
				"result_char":     string(result[i]),
				"context_before":  expected[max(0, i-3):i],
				"context_after":   expected[i+1 : min(len(expected), i+4)],
			}
		}
	}

	if len(expected) != len(result) {
		return map[string]any{
			"position": minLen,
			"reason":   "length mismatch",
			"expected": len(expected),
			"result":   len(result),
		}
	}

	return map[string]any{
		"position": -1,
		"reason":   "no mismatch found",
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

