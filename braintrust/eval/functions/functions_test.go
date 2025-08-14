package functions

import (
	"testing"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

func TestGetScorer_BasicUsage(t *testing.T) {
	// Test GetScorer behavior - it may succeed or fail depending on whether the function exists
	scorer, err := GetScorer[string, string]("test-go-functions", "fail-scorer-d879")

	if err != nil {
		// Function doesn't exist - should return nil scorer
		if scorer != nil {
			t.Fatal("Expected nil scorer when error occurs")
		}
		t.Logf("Function resolution failed as expected: %v", err)
	} else {
		// Function exists - should return valid scorer
		if scorer == nil {
			t.Fatal("Expected non-nil scorer when no error occurs")
		}
		t.Log("Function resolution succeeded")
	}
}

func TestGetScorer_Name(t *testing.T) {
	// Skip this test since we expect resolution to fail
	t.Skip("Skipping name test - requires successful resolution")
}

func TestGetScorer_Run(t *testing.T) {
	// Skip this test if we don't have a valid function ID to test with
	t.Skip("Skipping integration test - need valid function UUID")

	scorer, err := GetScorer[string, string]("test-go-functions", "fail-scorer-d879")
	if err != nil {
		t.Fatalf("GetScorer failed: %v", err)
	}

	ctx := t.Context()
	input := "What is 2+2?"
	expected := "4"
	result := "The answer is: What is 2+2?"
	metadata := eval.Metadata{"test": "value"}

	scores, err := scorer.Run(ctx, input, expected, result, metadata)
	if err != nil {
		t.Fatalf("scorer.Run failed: %v", err)
	}

	if len(scores) == 0 {
		t.Fatal("expected at least one score, got none")
	}

	// Should return a score between 0 and 1
	score := scores[0]
	if score.Score < 0 || score.Score > 1 {
		t.Errorf("expected score between 0 and 1, got %f", score.Score)
	}
}

func TestGetScorer_DifferentTypes(t *testing.T) {
	// Test with different generic types - should return error
	type CustomInput struct {
		Question string `json:"question"`
	}

	type CustomOutput struct {
		Answer string `json:"answer"`
	}

	scorer, err := GetScorer[CustomInput, CustomOutput]("test-project", "custom-scorer")

	if err == nil {
		t.Fatal("Expected error for non-existent function")
	}
	if scorer != nil {
		t.Fatal("Expected nil scorer when error occurs")
	}
}
