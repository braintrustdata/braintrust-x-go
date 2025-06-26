package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvokeFunctionRequest_Marshal(t *testing.T) {
	req := InvokeFunctionRequest{
		Input:    "test input",
		Expected: "expected output",
		Metadata: map[string]interface{}{
			"key": "value",
		},
		Tags: []string{"tag1", "tag2"},
		Parent: &TracingInfo{
			Type:     "experiment",
			ID:       "test-id",
			ObjectID: "object-id",
		},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test input")
	assert.Contains(t, string(data), "expected output")
	assert.Contains(t, string(data), "experiment")
}

func TestInvokeFunctionResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"output": "test output",
		"metadata": {"score": 0.8},
		"tags": ["success"],
		"metrics": {"duration": 100}
	}`

	var resp InvokeFunctionResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "test output", resp.Output)
	assert.Equal(t, 0.8, resp.Metadata["score"])
	assert.Equal(t, []string{"success"}, resp.Tags)
	assert.Equal(t, float64(100), resp.Metrics["duration"])
}

func TestTracingInfo_Marshal(t *testing.T) {
	info := TracingInfo{
		Type:      "project",
		ID:        "proj-123",
		ObjectID:  "obj-456",
		ComputeID: "comp-789",
	}

	data, err := json.Marshal(info)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "project")
	assert.Contains(t, string(data), "proj-123")
}

func TestInvokeFunctionRequest_OptionalFields(t *testing.T) {
	// Test with minimal required fields
	req := InvokeFunctionRequest{
		Input: map[string]string{"query": "hello"},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	// Verify optional fields are omitted when empty
	assert.NotContains(t, string(data), "expected")
	assert.NotContains(t, string(data), "metadata")
	assert.NotContains(t, string(data), "parent")
}

func TestInvokeFunctionRequest_WithAllFields(t *testing.T) {
	// Test request with all fields populated
	stream := true
	mode := "parallel"
	strict := false
	version := "1.0"
	globalFunction := "TestFunction"
	projectName := "TestProject"
	slug := "test-slug"

	req := InvokeFunctionRequest{
		Input:    map[string]string{"query": "hello world"},
		Expected: "expected response",
		Metadata: map[string]interface{}{"source": "test"},
		Tags:     []string{"test", "api"},
		Messages: []interface{}{
			map[string]string{"role": "user", "content": "test message"},
		},
		Parent: &TracingInfo{
			Type:      "experiment",
			ID:        "exp-123",
			ObjectID:  "obj-456",
			ComputeID: "comp-789",
		},
		Stream:         &stream,
		Mode:           &mode,
		Strict:         &strict,
		Version:        &version,
		GlobalFunction: &globalFunction,
		ProjectName:    &projectName,
		Slug:           &slug,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	// Verify all fields are present
	assert.Contains(t, string(data), "hello world")
	assert.Contains(t, string(data), "expected response")
	assert.Contains(t, string(data), "source")
	assert.Contains(t, string(data), "test")
	assert.Contains(t, string(data), "api")
	assert.Contains(t, string(data), "experiment")
	assert.Contains(t, string(data), "parallel")
	assert.Contains(t, string(data), "1.0")
	assert.Contains(t, string(data), "TestFunction")
	assert.Contains(t, string(data), "TestProject")
	assert.Contains(t, string(data), "test-slug")
}
