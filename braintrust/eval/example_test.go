package eval_test

import (
	"context"
	"log"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func Example() {
	// Set up tracing (requires BRAINTRUST_API_KEY)
	// export BRAINTRUST_API_KEY="your-api-key-here"
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// This task is hardcoded but usually you'd call an AI model here.
	greetingTask := func(ctx context.Context, input string) (string, error) {
		return "Hello " + input, nil
	}

	// Define your scoring function - returns a list of scores
	exactMatch := func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
		if expected == result {
			return eval.S(1.0), nil // Perfect match - S() is a helper for single scores
		}
		return eval.S(0.0), nil // No match
	}

	// Create and run the evaluation
	experimentID, err := eval.ResolveProjectExperimentID("greeting-experiment-v1", "my-ai-project")
	if err != nil {
		log.Fatal(err)
	}

	evaluation := eval.New(experimentID,
		eval.NewCases([]eval.Case[string, string]{
			{Input: "World", Expected: "Hello World"},
			{Input: "Alice", Expected: "Hello Alice"},
			{Input: "Bob", Expected: "Hello Bob"},
		}),
		greetingTask,
		[]eval.Scorer[string, string]{
			eval.NewScorer("exact_match", exactMatch),
		},
	)

	err = evaluation.Run(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Evaluation completed successfully!")
}

func ExampleNew() {
	// Simple doubling task
	task := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}

	// Test cases
	cases := eval.NewCases([]eval.Case[int, int]{
		{Input: 2, Expected: 4},
		{Input: 5, Expected: 10},
	})

	// Scorer
	scorers := []eval.Scorer[int, int]{
		eval.NewScorer("equals", func(ctx context.Context, input, expected, result int, _ eval.Metadata) (eval.Scores, error) {
			if expected == result {
				return eval.S(1.0), nil
			}
			return eval.S(0.0), nil
		}),
	}

	// Create evaluation
	evaluation := eval.New("exp-123", cases, task, scorers)

	err := evaluation.Run(context.Background())
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	log.Println("Done!")
}

func ExampleNewScorer() {
	// Single score scorer using S() helper
	equals := eval.NewScorer("equals", func(ctx context.Context, input, expected, result int, _ eval.Metadata) (eval.Scores, error) {
		if expected == result {
			return eval.S(1.0), nil
		}
		return eval.S(0.0), nil
	})

	// Multiple scores from one scorer
	analysis := eval.NewScorer("analysis", func(ctx context.Context, input, expected, result int, _ eval.Metadata) (eval.Scores, error) {
		exactScore := 0.0
		if expected == result {
			exactScore = 1.0
		}

		closeScore := 0.0
		if expected-result >= -1 && expected-result <= 1 {
			closeScore = 0.8
		}

		return eval.Scores{
			{Name: "exact", Score: exactScore},
			{Name: "close", Score: closeScore},
		}, nil
	})

	log.Printf("Created scorers: %s and %s", equals.Name(), analysis.Name())
}
