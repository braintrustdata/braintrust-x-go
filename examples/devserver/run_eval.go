package main

import (
	"context"
	"log"
	"strings"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	// Initialize tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Failed to start tracing: %v", err)
	}
	defer teardown()

	// Define the uppercase task
	uppercaseTask := func(ctx context.Context, input string) (string, error) {
		return strings.ToUpper(input), nil
	}

	// Define a scorer
	lengthScorer := eval.NewScorer("length", func(ctx context.Context, input, expected, result string, meta eval.Metadata) (eval.Scores, error) {
		score := float64(len(result)) / 10.0
		if score > 1.0 {
			score = 1.0
		}
		return eval.S(score), nil
	})

	// Run the evaluation using the dataset we created
	result, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: "uppercase-eval",
		Dataset:    "uppercase-test-data", // Use the dataset we created
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{lengthScorer},
	})
	if err != nil {
		log.Fatalf("Eval failed: %v", err)
	}

	log.Printf("Eval completed successfully!")
	if permalink, err := result.Permalink(); err == nil {
		log.Printf("View results: %s", permalink)
	}
}
