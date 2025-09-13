// This example shows how to use http scorers (e.g. scorers defined on
// braintrust.dev) and code scorers.

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

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

	// Simple task that just returns the input doubled
	task := func(ctx context.Context, input int) (int, error) {
		return input * 2, nil
	}

	// Static test cases
	cases := []eval.Case[int, int]{
		{Input: 5, Expected: 10},
		{Input: 3, Expected: 6},
		{Input: 7, Expected: 14},
	}

	// an example of a scorer that returns a single number
	randomScorer := eval.NewScorer[int, int]("random", func(ctx context.Context, input, expected, result int, _ eval.Metadata) (eval.Scores, error) {
		score := rand.Float64()
		return eval.S(score), nil
	})

	// an example of a scorer that returns more than one score.
	listScorer := eval.NewScorer[int, int]("list", func(ctx context.Context, input, expected, result int, _ eval.Metadata) (eval.Scores, error) {
		return eval.Scores{
			{Name: "poor", Score: 0},
			{Name: "average", Score: 0.5},
			{Name: "excellent", Score: 1},
		}, nil
	})

	// Build scorers list with local scorers
	scorers := []eval.Scorer[int, int]{
		autoevals.NewEquals[int, int](),
		randomScorer,
		listScorer,
	}

	// Try to get online scorer - add if available
	onlineScorer, err := functions.GetScorer[int, int]("test-go-functions", "fail-scorer-d879")
	if err != nil {
		log.Printf("⚠️ Online scorer not available: %v", err)
		log.Println("📊 Running with local scorers only...")
	} else {
		log.Println("✅ Online scorer available, adding to list")
		scorers = append(scorers, onlineScorer)
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
