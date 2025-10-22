// This example demonstrates using the Update option to append results to an existing experiment.
// This is useful for iterative testing where you want to add more test cases to an existing
// evaluation run rather than creating a new experiment each time.
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	teardown, err := trace.Quickstart(braintrust.WithDefaultProject("go-sdk-examples"))
	if err != nil {
		log.Fatalf("Error starting trace: %v", err)
	}
	defer teardown()

	// Simple task: convert text to uppercase
	uppercaseTask := func(ctx context.Context, input string) (string, error) {
		return strings.ToUpper(input), nil
	}

	// Simple scorer: check if result is uppercase
	isUppercaseScorer := eval.NewScorer("is_uppercase", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
		if result == strings.ToUpper(result) {
			return eval.S(1.0), nil
		}
		return eval.S(0.0), nil
	})

	// Round 1: Create a new experiment with initial test cases
	log.Println("Round 1: Creating new experiment")
	firstCases := []eval.Case[string, string]{
		{Input: "round 1: hello", Expected: "ROUND 1: HELLO"},
		{Input: "round 1: world", Expected: "ROUND 1: WORLD"},
		{Input: "round 1: test", Expected: "ROUND 1: TEST"},
	}

	result1, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: "uppercase-eval-demo",
		Cases:      eval.NewCases(firstCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     false, // Create new experiment (default behavior)
	})
	if err != nil {
		log.Fatalf("Round 1 failed: %v", err)
	}

	// IMPORTANT: Capture the experiment name from the first run.
	// Updates should use the unique name of experiments.
	// If an experiment with this name already exists, the API will add a random suffix,
	// so we need to use this exact name (including the suffix) for subsequent updates.
	experimentName := result1.Name()
	experimentID := result1.ID()

	permalink1, _ := result1.Permalink()
	log.Printf("Round 1 complete: %s\n", permalink1)

	// Wait a bit between rounds to make it easier to see the updates
	time.Sleep(2 * time.Second)

	// Round 2: Update the existing experiment with additional test cases
	log.Println("Round 2: Appending to experiment")
	secondCases := []eval.Case[string, string]{
		{Input: "round 2: append", Expected: "ROUND 2: APPEND"},
		{Input: "round 2: update", Expected: "ROUND 2: UPDATE"},
	}

	result2, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: experimentName, // Use the EXACT name from Round 1 (including any suffix)
		Cases:      eval.NewCases(secondCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     true, // Append to existing experiment
	})
	if err != nil {
		log.Fatalf("Round 2 failed: %v", err)
	}

	// Verify we're using the same experiment
	if result2.ID() != experimentID {
		log.Fatalf("ERROR: Round 2 created a different experiment! Expected %s, got %s", experimentID, result2.ID())
	}

	permalink2, _ := result2.Permalink()
	log.Printf("Round 2 complete: %s\n", permalink2)

	time.Sleep(2 * time.Second)

	// Round 3: Add even more cases to the same experiment
	log.Println("Round 3: Appending to experiment")
	thirdCases := []eval.Case[string, string]{
		{Input: "round 3: continue", Expected: "ROUND 3: CONTINUE"},
		{Input: "round 3: testing", Expected: "ROUND 3: TESTING"},
	}

	result3, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: experimentName, // Use the EXACT name from Round 1 (including any suffix)
		Cases:      eval.NewCases(thirdCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     true, // Continue appending to the same experiment
	})
	if err != nil {
		log.Fatalf("Round 3 failed: %v", err)
	}

	// Verify we're using the same experiment
	if result3.ID() != experimentID {
		log.Fatalf("ERROR: Round 3 created a different experiment! Expected %s, got %s", experimentID, result3.ID())
	}

	permalink3, _ := result3.Permalink()
	log.Printf("Round 3 complete: %s", permalink3)
}
