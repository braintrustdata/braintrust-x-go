package testspan

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTestSpan(t *testing.T) {
	// Create test data
	now := time.Now()
	start := now.Add(-10 * time.Minute)
	end := now

	// Create a test span stub
	inputJSON, _ := json.Marshal(map[string]string{"query": "What is 2+2?"})
	outputJSON, _ := json.Marshal(map[string]string{"answer": "4"})
	metadataJSON, _ := json.Marshal(map[string]interface{}{"model": "gpt-4", "provider": "openai"})
	metricsJSON, _ := json.Marshal(map[string]float64{"tokens": 10, "prompt_tokens": 5})

	attrs := []attr.KeyValue{
		attr.String("braintrust.input", string(inputJSON)),
		attr.String("braintrust.output", string(outputJSON)),
		attr.String("braintrust.metadata", string(metadataJSON)),
		attr.String("braintrust.metrics", string(metricsJSON)),
		attr.String("custom.key", "custom value"),
	}

	stub := tracetest.SpanStub{
		Name:       "test.span",
		StartTime:  start.Add(time.Second),
		EndTime:    end.Add(-time.Second),
		Attributes: attrs,
	}

	// Create TestSpan
	testSpan := New(t, stub)

	testSpan.AssertNameIs("test.span")
	testSpan.AssertTimingIsValid(start, end)

	// Test Attr
	found, val := testSpan.Attr("custom.key")
	assert.True(t, found)
	assert.Equal(t, "custom value", val.AsString())

	found, _ = testSpan.Attr("non.existent")
	assert.False(t, found)

	// Test AttrMust
	val = testSpan.AttrMust("custom.key")
	assert.Equal(t, "custom value", val.AsString())

	// Test Input
	input := testSpan.Input()
	inputMap, ok := input.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "What is 2+2?", inputMap["query"])

	// Test Output
	output := testSpan.Output()
	outputMap, ok := output.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "4", outputMap["answer"])

	// Test Metadata
	metadata := testSpan.Metadata()
	assert.Equal(t, "gpt-4", metadata["model"])
	assert.Equal(t, "openai", metadata["provider"])

	// Test Metrics
	metrics := testSpan.Metrics()
	assert.Equal(t, 10.0, metrics["tokens"])
	assert.Equal(t, 5.0, metrics["prompt_tokens"])
}
