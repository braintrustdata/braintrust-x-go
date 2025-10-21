// This example demonstrates using the Update option to append results to an existing experiment.
// This is useful for iterative testing where you want to add more test cases to an existing
// evaluation run rather than creating a new experiment each time.
package main

import (
	"context"
	"log"
	"strings"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	log.Println("🔄 Demonstrating Eval Update Feature")
	log.Println("=====================================")

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

	experimentName := "uppercase-eval-demo"

	// First run: Create a new experiment with initial test cases
	log.Println("\n📝 Run 1: Creating new experiment with 3 test cases...")
	firstCases := []eval.Case[string, string]{
		{Input: "hello", Expected: "HELLO"},
		{Input: "world", Expected: "WORLD"},
		{Input: "test", Expected: "TEST"},
	}

	result1, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: experimentName,
		Cases:      eval.NewCases(firstCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     false, // Create new experiment (default behavior)
	})
	if err != nil {
		log.Printf("⚠️  First run completed with issues: %v", err)
	} else {
		log.Println("✅ First run completed successfully!")
	}

	if result1 != nil {
		permalink1, _ := result1.Permalink()
		log.Printf("🔗 First run experiment link: %s", permalink1)
	}

	// Second run: Update the existing experiment with additional test cases
	log.Println("\n📝 Run 2: Updating existing experiment with 2 more test cases...")
	secondCases := []eval.Case[string, string]{
		{Input: "append", Expected: "APPEND"},
		{Input: "update", Expected: "UPDATE"},
	}

	result2, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: experimentName,
		Cases:      eval.NewCases(secondCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     true, // Append to existing experiment
	})
	if err != nil {
		log.Printf("⚠️  Second run completed with issues: %v", err)
	} else {
		log.Println("✅ Second run completed successfully!")
	}

	if result2 != nil {
		permalink2, _ := result2.Permalink()
		log.Printf("🔗 Second run experiment link: %s", permalink2)
	}

	// Third run: Add even more cases to the same experiment
	log.Println("\n📝 Run 3: Updating existing experiment with 2 more test cases...")
	thirdCases := []eval.Case[string, string]{
		{Input: "continue", Expected: "CONTINUE"},
		{Input: "testing", Expected: "TESTING"},
	}

	result3, err := eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: experimentName,
		Cases:      eval.NewCases(thirdCases),
		Task:       uppercaseTask,
		Scorers:    []eval.Scorer[string, string]{isUppercaseScorer},
		Update:     true, // Continue appending to the same experiment
	})
	if err != nil {
		log.Printf("⚠️  Third run completed with issues: %v", err)
	} else {
		log.Println("✅ Third run completed successfully!")
	}

	if result3 != nil {
		permalink3, _ := result3.Permalink()
		log.Printf("🔗 Third run experiment link: %s", permalink3)
		log.Println("\n🎉 All three runs appended to the same experiment!")
		log.Println("   Check the Braintrust UI to see all 7 test cases in a single experiment.")
	}

	log.Println("\n💡 Key Takeaways:")
	log.Println("   - Update: false → Creates a new experiment")
	log.Println("   - Update: true  → Appends to existing experiment")
	log.Println("   - Useful for iterative testing and adding more test cases over time")
}
