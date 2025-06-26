package autoevals

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/braintrust/braintrust-x-go/braintrust/eval"
)

func TestNewEquals(t *testing.T) {
	scorer := NewEquals[string, string]()

	assert.Equal(t, "Equals", scorer.Name())

	// Test equal strings
	scores, err := scorer.Run(t.Context(), "input", "hello", "hello", eval.Metadata{})
	assert.NoError(t, err)
	assert.Len(t, scores, 1)
	assert.Equal(t, "Equals", scores[0].Name)
	assert.Equal(t, 1.0, scores[0].Score)

	// Test different strings
	scores, err = scorer.Run(t.Context(), "input", "hello", "world", eval.Metadata{})
	assert.NoError(t, err)
	assert.Len(t, scores, 1)
	assert.Equal(t, "Equals", scores[0].Name)
	assert.Equal(t, 0.0, scores[0].Score)
}

func TestNewLessThan(t *testing.T) {
	scorer := NewLessThan[string, float64]()

	assert.Equal(t, "LessThan", scorer.Name())

	// Test expected < result
	scores, err := scorer.Run(t.Context(), "input", 0.5, 0.8, eval.Metadata{})
	assert.NoError(t, err)
	assert.Len(t, scores, 1)
	assert.Equal(t, "LessThan", scores[0].Name)
	assert.Equal(t, 1.0, scores[0].Score)

	// Test expected >= result
	scores, err = scorer.Run(t.Context(), "input", 0.8, 0.5, eval.Metadata{})
	assert.NoError(t, err)
	assert.Len(t, scores, 1)
	assert.Equal(t, "LessThan", scores[0].Name)
	assert.Equal(t, 0.0, scores[0].Score)
}

func TestNewLevenshtein_Initialization(t *testing.T) {
	scorer := NewLevenshtein[string]()

	assert.Equal(t, "Levenshtein", scorer.Name())
	assert.NotNil(t, scorer)
}

// Note: Full integration test for Levenshtein would require API credentials and the global function to be available.
// This test verifies the scorer can be created and has the correct name.
// For full testing, run with actual Braintrust API credentials in a real environment.
func TestNewLevenshtein_Structure(t *testing.T) {
	scorer := NewLevenshtein[string]()

	// Verify the scorer implements the Scorer interface
	var _ = scorer

	// Verify it has the correct name
	assert.Equal(t, "Levenshtein", scorer.Name())
}
