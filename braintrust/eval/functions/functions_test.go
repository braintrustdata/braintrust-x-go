package functions

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProjectName = "test-go-functions"

func uniqueFuncName(baseName string) string {
	// Use current time and random number to ensure uniqueness across processes
	timestamp := time.Now().Unix()
	randomSuffix := rand.Intn(100000)
	return fmt.Sprintf("%s-%d-%05d", baseName, timestamp, randomSuffix)
}

func TestScorerFunctionality(t *testing.T) {
	assert := assert.New(t)
	functionData := map[string]any{"type": "prompt"}
	functionSlug := uniqueFuncName("test-scorer-functionality")

	// Create a test function
	functionID, err := createFunction(testProjectName, "Test Scorer Function", functionSlug, "A test function to verify all scorer functionality", functionData)
	require.NoError(t, err)
	require.NotEmpty(t, functionID)

	// Test GetScorer
	scorer, err := GetScorer[string, string](testProjectName, functionSlug)
	assert.NoError(err)
	assert.NotNil(scorer)
	assert.Equal("Test Scorer Function", scorer.Name())

	// Test QueryScorer with different option patterns
	queryScorer, err := QueryScorer[string, string](Opts{ProjectName: testProjectName, Slug: functionSlug})
	assert.NoError(err)
	assert.NotNil(queryScorer)
	assert.Equal("Test Scorer Function", queryScorer.Name())

	// Test QueryScorer with function ID directly (uses function ID as name)
	queryScorer2, err := QueryScorer[string, string](Opts{FunctionID: functionID})
	assert.NoError(err)
	assert.NotNil(queryScorer2)
	assert.Equal(functionID, queryScorer2.Name()) // When using function ID directly, name = ID

	// Test QueryScorers
	scorers, err := QueryScorers[string, string](Opts{ProjectName: testProjectName, Slug: functionSlug, Limit: 1})
	assert.NoError(err)
	assert.Len(scorers, 1)
	assert.Equal("Test Scorer Function", scorers[0].Name())
}

func TestScorerRun(t *testing.T) {
	assert := assert.New(t)
	functionSlug := uniqueFuncName("test-go-scorer-copy")
	promptData := map[string]any{
		"parser": map[string]any{
			"type": "llm_classifier", "use_cot": true,
			"choice_scores": map[string]any{"fail": 0.31, "pass": 0.32, "mid": 0.33},
		},
		"prompt": map[string]any{
			"type": "chat",
			"messages": []map[string]any{
				{"role": "system", "content": "You are a scorer. Evaluate the input and output."},
				{"role": "user", "content": "Choose 'pass' if the output is good, 'mid' if it's okay, 'fail' if it's bad. For this test, always choose 'pass'."},
			},
		},
		"options": map[string]any{
			"model":  "gpt-4",
			"params": map[string]any{"use_cache": true, "temperature": 0},
		},
	}

	functionID, err := createFunctionWithPromptData(testProjectName, "Test Go Scorer Copy", functionSlug, "A test scorer copied from fail-scorer structure", promptData)
	require.NoError(t, err)
	require.NotEmpty(t, functionID)

	scorer, err := GetScorer[string, string](testProjectName, functionSlug)
	assert.NoError(err)

	scores, err := scorer.Run(context.Background(), "test input", "expected output", "actual output", map[string]any{"test": "value"})
	assert.NoError(err)
	assert.Len(scores, 1) // Assert we received exactly one score

	score := scores[0]
	assert.Equal("Test Go Scorer Copy", score.Name)
	assert.True(score.Score >= 0.31)
	assert.True(score.Score <= 0.33)
}

func TestVersionHandling(t *testing.T) {
	assert := assert.New(t)
	functionData := map[string]any{"type": "prompt"}
	functionSlug := uniqueFuncName("test-version-handling")

	// Create first version
	functionID1, err := createFunction(testProjectName, "Version Test v1", functionSlug, "First version", functionData)
	require.NoError(t, err)
	require.NotEmpty(t, functionID1)

	// Verify first version is accessible
	scorer1, err := GetScorer[string, string](testProjectName, functionSlug)
	assert.NoError(err)
	assert.Equal("Version Test v1", scorer1.Name())

	// Create second version (same slug, different name) - this should replace the first
	functionID2, err := createFunction(testProjectName, "Version Test v2", functionSlug, "Second version - updated", functionData)
	assert.NoError(err)
	assert.NotEmpty(functionID2)
	// Note: API uses "create or replace" behavior, so same slug = same function ID

	// GetScorer should return the updated version (v2)
	scorer2, err := GetScorer[string, string](testProjectName, functionSlug)
	assert.NoError(err)
	assert.Equal("Version Test v2", scorer2.Name())

	// QueryScorer should also return the updated version (v2)
	queryScorer, err := QueryScorer[string, string](Opts{ProjectName: testProjectName, Slug: functionSlug})
	assert.NoError(err)
	assert.Equal("Version Test v2", queryScorer.Name())

	// QueryScorers should return the updated function
	scorers, err := QueryScorers[string, string](Opts{ProjectName: testProjectName, Slug: functionSlug})
	assert.NoError(err)
	assert.Len(scorers, 1) // Should be exactly one function (replaced, not versioned)
	assert.Equal("Version Test v2", scorers[0].Name())

	// Test that we can create functions with different slugs and query multiple
	functionID3, err := createFunction(testProjectName, "Another Function", uniqueFuncName("test-version-handling-2"), "Different slug", functionData)
	assert.NoError(err)
	assert.NotEmpty(functionID3)

	// QueryScorers with project name should return multiple functions
	allProjectScorers, err := QueryScorers[string, string](Opts{ProjectName: testProjectName, Limit: 10})
	assert.NoError(err)
	assert.GreaterOrEqual(len(allProjectScorers), 2) // At least our two test functions
}

func TestGetScorer_DifferentTypes(t *testing.T) {
	assert := assert.New(t)
	type CustomInput struct {
		Question string `json:"question"`
	}
	type CustomOutput struct {
		Answer string `json:"answer"`
	}

	scorer, err := GetScorer[CustomInput, CustomOutput](testProjectName, "non-existent-scorer")
	assert.Error(err)
	assert.Nil(scorer)
}
