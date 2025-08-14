// Simple example to test online scorers with static data
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
	log.Println("🧪 Testing Online Scorers")
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("❌ Failed to start trace: %v", err)
	}
	defer teardown()

	// Simple task that just echoes the input
	task := func(ctx context.Context, input string) (string, error) {
		return "Echo: " + input, nil
	}

	// Static test cases
	cases := []eval.Case[string, string]{
		{Input: "hello", Expected: "hello"},
		{Input: "world", Expected: "world"},
		{Input: "test", Expected: "test"},
	}

	// Try to create online scorer - will fail for non-existent function
	onlineScorer, err := functions.GetScorer[string, string]("test-go-functions", "fail-scorer-d879")
	if err != nil {
		log.Printf("⚠️ Failed to create online scorer: %v", err)
		log.Println("📝 Continuing with local scorers only...")
	}

	// Build scorers list
	scorers := []eval.Scorer[string, string]{
		autoevals.NewEquals[string, string](),
	}

	// Add online scorer if it was created successfully
	if onlineScorer != nil {
		scorers = append(scorers, onlineScorer)
		log.Println("✅ Online scorer added successfully")
	}

	// Create evaluation
	experimentID, err := eval.ResolveProjectExperimentID("test-go-functions", "test-go-functions")
	if err != nil {
		log.Fatalf("❌ Failed to resolve experiment: %v", err)
	}

	evaluation := eval.New(experimentID, eval.NewCases(cases), task, scorers)

	log.Println("🚀 Running evaluation...")
	err = evaluation.Run(context.Background())
	if err != nil {
		log.Printf("⚠️ Eval completed with errors: %v", err)
	} else {
		log.Println("✅ Eval completed successfully")
	}

	fmt.Println("Done! Check the Braintrust UI to see results.")
}
