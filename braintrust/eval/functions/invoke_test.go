package functions

import (
	"context"
	"os"
	"testing"
)

func TestInvoke(t *testing.T) {
	// Skip if no API key
	if os.Getenv("BRAINTRUST_API_KEY") == "" {
		t.Skip("Skipping integration test: BRAINTRUST_API_KEY not set")
	}

	ctx := context.Background()

	t.Run("invoke with function ID", func(t *testing.T) {
		// Use a known test function ID or skip
		functionID := os.Getenv("TEST_FUNCTION_ID")
		if functionID == "" {
			t.Skip("Skipping: TEST_FUNCTION_ID not set")
		}

		result, err := invoke(ctx, invokeOptions{
			FunctionID: functionID,
			Input: map[string]any{
				"test": "data",
			},
		})

		if err != nil {
			t.Fatalf("invoke() error = %v", err)
		}

		if result == nil {
			t.Error("invoke() returned nil result")
		}

		t.Logf("invoke result: %v", result)
	})

	t.Run("invoke with invalid function returns error", func(t *testing.T) {
		_, err := invoke(ctx, invokeOptions{
			Project: "nonexistent-project",
			Slug:    "nonexistent-function",
			Input:   map[string]any{"test": "data"},
		})

		if err == nil {
			t.Error("Expected error for nonexistent function, got nil")
		}
	})
}
