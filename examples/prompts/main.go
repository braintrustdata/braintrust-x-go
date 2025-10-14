// This example demonstrates using a Braintrust prompt in an evaluation.
//
// To run this example:
//  1. Set BRAINTRUST_API_KEY environment variable
//  2. Create a prompt in Braintrust with slug "sdk-greeter-prompt-195e" in the "go-sdk-examples" project
//  3. Run: go run examples/prompts/main.go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/braintrustdata/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval/functions"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	fmt.Println("Example: Using a Braintrust prompt in an evaluation")
	fmt.Println("Note: This example requires a prompt with slug 'sdk-greeter-prompt-195e' to exist in the 'go-sdk-examples' project")
	fmt.Println()

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Error starting trace: %v", err)
	}
	defer teardown()

	ctx := context.Background()

	// Example dataset with test cases
	cases := []eval.Case[string, string]{
		{
			Input:    "Joe",
			Expected: "Hi Joe",
		},
		{
			Input:    "Jane",
			Expected: "Hello Jane",
		},
		{
			Input:    "Bob",
			Expected: "Hi Bob",
		},
	}

	// Run evaluation using a hosted prompt
	_, err = eval.Run(ctx, eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: "greeter-test",
		Cases:      eval.NewCases(cases),

		// Use GetTask to create a task from a hosted prompt
		// The prompt will be invoked for each test case with the input data
		Task: functions.GetTask[string, string](functions.Opts{
			Project: "go-sdk-examples",
			Slug:    "sdk-greeter-prompt-195e",
			// Environment: "production",
			// Optional: specify version
			// Version: "v1.2.3",
		}),

		// Add scorers - using autoevals Equals scorer
		Scorers: []eval.Scorer[string, string]{
			autoevals.NewEquals[string, string](),
		},
	})

	if err != nil {
		log.Fatalf("Eval failed: %v", err)
	}
}
