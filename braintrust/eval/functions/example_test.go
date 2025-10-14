package functions_test

import (
	"context"
	"log"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval/functions"
)

// Example demonstrates using a hosted Braintrust prompt and scorer in an evaluation.
func Example() {
	ctx := context.Background()

	// Create test cases
	cases := []eval.Case[string, string]{
		{Input: "Joe", Expected: "Hi Joe"},
		{Input: "Jane", Expected: "Hello Jane"},
	}

	// Get a hosted scorer
	scorer, err := functions.GetScorer[string, string]("my-project", "my-scorer")
	if err != nil {
		log.Fatalf("Failed to get scorer: %v", err)
	}

	// Run evaluation using both a hosted prompt (task) and hosted scorer
	_, err = eval.Run(ctx, eval.Opts[string, string]{
		Project:    "my-project",
		Experiment: "greeter-test",
		Cases:      eval.NewCases(cases),

		// Use GetTask to create a task from a hosted prompt
		Task: functions.GetTask[string, string](functions.Opts{
			Project: "my-project",
			Slug:    "greeter-prompt",
			// Optional: specify version or environment
			// Version: "v1.2.3",
			// Environment: "production",
		}),

		// Use the hosted scorer
		Scorers: []eval.Scorer[string, string]{scorer},
	})

	if err != nil {
		log.Fatalf("Eval failed: %v", err)
	}
}
