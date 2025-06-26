package traceanthropic

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"

	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
)

func TestMiddleware(t *testing.T) {
	// Create a test request with a Messages API call
	requestBody := `{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": "Hello, Claude!"
			}
		]
	}`

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create a mock response
	responseBody := `{
		"id": "msg_01Aq9w938a90dw8q",
		"type": "message",
		"role": "assistant",
		"content": [
			{
				"type": "text",
				"text": "Hello! How can I help you today?"
			}
		],
		"model": "claude-3-haiku-20240307",
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {
			"input_tokens": 12,
			"output_tokens": 9
		}
	}`

	// Create a mock next middleware
	next := func(req *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}

	// Call the middleware
	resp, err := Middleware(req, next)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Read the response body to trigger the tracer
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Hello! How can I help you today?")

	// Close the response body to trigger completion
	err = resp.Body.Close()
	require.NoError(t, err)
}

func TestMessagesTracer(t *testing.T) {
	tracer := newMessagesTracer()
	assert.NotNil(t, tracer)
	assert.False(t, tracer.streaming)
	assert.Equal(t, "anthropic", tracer.metadata["provider"])
	assert.Equal(t, "/v1/messages", tracer.metadata["endpoint"])

	// Test StartSpan
	requestBody := `{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": "Hello, Claude!"
			}
		],
		"stream": false
	}`

	ctx := t.Context()
	start := time.Now()
	reader := strings.NewReader(requestBody)

	newCtx, span, err := tracer.StartSpan(ctx, start, reader)
	require.NoError(t, err)
	require.NotNil(t, span)
	require.NotNil(t, newCtx)

	// Verify metadata was parsed
	assert.Equal(t, "claude-3-haiku-20240307", tracer.metadata["model"])
	assert.Equal(t, float64(1024), tracer.metadata["max_tokens"])
	assert.Equal(t, false, tracer.metadata["stream"])
	assert.False(t, tracer.streaming)
}

func TestMessagesTracerStreaming(t *testing.T) {
	tracer := newMessagesTracer()

	// Test streaming request
	requestBody := `{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": "Hello, Claude!"
			}
		],
		"stream": true
	}`

	ctx := t.Context()
	start := time.Now()
	reader := strings.NewReader(requestBody)

	_, span, err := tracer.StartSpan(ctx, start, reader)
	require.NoError(t, err)
	require.NotNil(t, span)

	// Verify streaming was detected
	assert.True(t, tracer.streaming)
	assert.Equal(t, true, tracer.metadata["stream"])
}

func TestParseUsageTokens(t *testing.T) {
	usage := map[string]interface{}{
		"input_tokens":  float64(12),
		"output_tokens": float64(9),
	}

	metrics := parseUsageTokens(usage)

	assert.Equal(t, int64(12), metrics["prompt_tokens"])
	assert.Equal(t, int64(9), metrics["completion_tokens"])
}

func TestParseUsageTokensWithCache(t *testing.T) {
	t.Run("cache_creation_input_tokens", func(t *testing.T) {
		usage := map[string]interface{}{
			"input_tokens":                float64(10),
			"output_tokens":               float64(5),
			"cache_creation_input_tokens": float64(100),
		}

		metrics := parseUsageTokens(usage)

		// Should include cache creation tokens in the total
		assert.Equal(t, int64(110), metrics["prompt_tokens"]) // 10 + 100
		assert.Equal(t, int64(5), metrics["completion_tokens"])
		assert.Equal(t, int64(100), metrics["prompt_cache_creation_tokens"])
	})

	t.Run("cache_read_input_tokens", func(t *testing.T) {
		usage := map[string]interface{}{
			"input_tokens":            float64(8),
			"output_tokens":           float64(12),
			"cache_read_input_tokens": float64(50),
		}

		metrics := parseUsageTokens(usage)

		// Should include cache read tokens in the total
		assert.Equal(t, int64(58), metrics["prompt_tokens"]) // 8 + 50
		assert.Equal(t, int64(12), metrics["completion_tokens"])
		assert.Equal(t, int64(50), metrics["prompt_cached_tokens"])
	})

	t.Run("both_cache_tokens", func(t *testing.T) {
		usage := map[string]interface{}{
			"input_tokens":                float64(15),
			"output_tokens":               float64(20),
			"cache_creation_input_tokens": float64(200),
			"cache_read_input_tokens":     float64(75),
		}

		metrics := parseUsageTokens(usage)

		// Should include both cache tokens in the total
		assert.Equal(t, int64(290), metrics["prompt_tokens"]) // 15 + 200 + 75
		assert.Equal(t, int64(20), metrics["completion_tokens"])
		assert.Equal(t, int64(200), metrics["prompt_cache_creation_tokens"])
		assert.Equal(t, int64(75), metrics["prompt_cached_tokens"])
	})

	t.Run("cache_tokens_without_input_tokens", func(t *testing.T) {
		usage := map[string]interface{}{
			"output_tokens":               float64(10),
			"cache_creation_input_tokens": float64(150),
			"cache_read_input_tokens":     float64(25),
		}

		metrics := parseUsageTokens(usage)

		// Should still account for cache tokens even without explicit input_tokens
		assert.Equal(t, int64(175), metrics["prompt_tokens"]) // 150 + 25
		assert.Equal(t, int64(10), metrics["completion_tokens"])
		assert.Equal(t, int64(150), metrics["prompt_cache_creation_tokens"])
		assert.Equal(t, int64(25), metrics["prompt_cached_tokens"])
	})
}

// TestMiddlewareIntegration tests the middleware with real Anthropic API calls
func TestMiddlewareIntegration(t *testing.T) {
	// Skip if no API key is available
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	// Set up test tracer and client
	client, exporter := setUpTest(t, apiKey)

	// Make a simple API call
	timer := oteltest.NewTimer()
	ctx := t.Context()
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model: anthropic.Model("claude-3-haiku-20240307"), // Use cheapest model
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is the capital of France?")),
		},
		MaxTokens: 1024, // Using higher token count - very low MaxTokens (like 10) cause timeouts
	})
	timeRange := timer.Tick()

	// Verify the API call succeeded
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Basic response validation
	assert.Equal(t, anthropic.Model("claude-3-haiku-20240307"), resp.Model)
	assert.Equal(t, "assistant", string(resp.Role))
	assert.NotEmpty(t, resp.Content)

	// Verify usage metrics are present
	assert.Greater(t, resp.Usage.InputTokens, int64(0))
	assert.Greater(t, resp.Usage.OutputTokens, int64(0))

	// Verify we got some content back
	require.Len(t, resp.Content, 1)
	assert.NotEmpty(t, resp.Content[0].Text)

	t.Logf("✅ API call successful. Response: %s", resp.Content[0].Text)
	t.Logf("📊 Usage - Input: %d, Output: %d", resp.Usage.InputTokens, resp.Usage.OutputTokens)

	// Validate spans were generated correctly
	span := exporter.FlushOne()
	assertSpanValid(t, span, timeRange)

	// Verify span content
	input := span.Attr("braintrust.input").String()
	assert.Contains(t, input, "What is the capital of France?")

	output := span.Output()
	assert.NotNil(t, output)

	metadata := span.Metadata()
	assert.Equal(t, "anthropic", metadata["provider"])
	assert.Equal(t, "claude-3-haiku-20240307", metadata["model"])
	assert.Equal(t, "/v1/messages", metadata["endpoint"])
	assert.Equal(t, float64(1024), metadata["max_tokens"])

	// assertSpanValid already validates all metrics comprehensively, just log for visibility
	metrics := span.Metrics()
	t.Logf("🎯 Span validation passed: %d metrics, %d metadata fields", len(metrics), len(metadata))
	t.Logf("📊 Non-streaming metrics - prompt: %.0f, completion: %.0f, cached: %.0f, cache_creation: %.0f",
		metrics["prompt_tokens"], metrics["completion_tokens"],
		metrics["prompt_cached_tokens"], metrics["prompt_cache_creation_tokens"])
}

// TestMiddlewareIntegrationStreaming tests the middleware with real Anthropic streaming API calls
func TestMiddlewareIntegrationStreaming(t *testing.T) {
	// Skip if no API key is available
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	// Set up test tracer and client
	client, exporter := setUpTest(t, apiKey)

	// Make a streaming API call
	timer := oteltest.NewTimer()
	ctx := t.Context()
	stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model: anthropic.Model("claude-3-haiku-20240307"), // Use cheapest model
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Tell me a very short joke.")),
		},
		MaxTokens:   512,
		Temperature: anthropic.Float(0.8),
		TopP:        anthropic.Float(0.95),
	})

	var completeText string

	// Iterate through the streaming response
	for stream.Next() {
		event := stream.Current()
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				completeText += deltaVariant.Text
			}
		}
	}
	require.NoError(t, stream.Err())
	timeRange := timer.Tick()

	// Basic response validation
	assert.NotEmpty(t, completeText)
	t.Logf("✅ Streaming API call successful. Complete response: %s", completeText)

	// Validate spans were generated correctly
	span := exporter.FlushOne()

	assertSpanValid(t, span, timeRange)

	// Verify span content
	input := span.Attr("braintrust.input").String()
	assert.Contains(t, input, "Tell me a very short joke.")

	output := span.Output()
	assert.NotNil(t, output)

	// The output should contain the complete streamed text in JSON format
	outputStr := span.Attr("braintrust.output").String()
	// For streaming, the output is stored as JSON: [{"text":"...", "type":"text"}]
	// So we check that both the accumulated text and the JSON contain expected content
	assert.Contains(t, outputStr, "joke")    // Should contain the word "joke"
	assert.Contains(t, completeText, "joke") // Ensure we got the text from streaming
	// Also verify that some of the streamed content matches what's in the span
	assert.Contains(t, outputStr, completeText[:10]) // Check first 10 chars are in the output

	metadata := span.Metadata()
	assert.Equal(t, "anthropic", metadata["provider"])
	assert.Equal(t, "claude-3-haiku-20240307", metadata["model"])
	assert.Equal(t, "/v1/messages", metadata["endpoint"])
	assert.Equal(t, float64(512), metadata["max_tokens"])
	assert.Equal(t, 0.8, metadata["temperature"])
	assert.Equal(t, 0.95, metadata["top_p"])
	assert.Equal(t, true, metadata["stream"]) // Should detect streaming mode

	// assertSpanValid already validates all metrics comprehensively, just log for visibility
	metrics := span.Metrics()
	t.Logf("🎯 Streaming span validation passed: %d metrics, %d metadata fields", len(metrics), len(metadata))
	t.Logf("📊 Streaming metrics - prompt: %.0f, completion: %.0f, cached: %.0f, cache_creation: %.0f",
		metrics["prompt_tokens"], metrics["completion_tokens"],
		metrics["prompt_cached_tokens"], metrics["prompt_cache_creation_tokens"])
}

// setUpTest is a helper function that sets up a new tracer provider for each test.
// It returns an anthropic client and an exporter.
func setUpTest(t *testing.T, apiKey string) (anthropic.Client, *oteltest.Exporter) {
	t.Helper()

	_, exporter := oteltest.Setup(t)

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithMiddleware(Middleware),
	)

	return client, exporter
}

// assertSpanValid asserts all the common properties of an Anthropic span are valid.
func assertSpanValid(t *testing.T, span oteltest.Span, timeRange oteltest.TimeRange) {
	t.Helper()
	assert := assert.New(t)

	span.AssertInTimeRange(timeRange)
	span.AssertNameIs("anthropic.messages.create")
	assert.Equal(codes.Unset, span.Stub.Status.Code)

	metadata := span.Metadata()
	assert.Equal("anthropic", metadata["provider"])
	assert.Equal("/v1/messages", metadata["endpoint"])

	// validate metrics
	metrics := span.Metrics()
	gtez := func(v float64) bool { return v >= 0 }
	gtz := func(v float64) bool { return v > 0 }

	// All expected metrics must be present - core metrics and cache metrics
	requiredMetrics := map[string]func(float64) bool{
		"prompt_tokens":                gtz,  // Should always be > 0
		"completion_tokens":            gtz,  // Should always be > 0
		"tokens":                       gtz,  // Should always be > 0
		"prompt_cached_tokens":         gtez, // Should always be ≥ 0 (even if 0)
		"prompt_cache_creation_tokens": gtez, // Should always be ≥ 0 (even if 0)
	}

	// First, ensure all required metrics are present
	for metricName := range requiredMetrics {
		assert.Contains(metrics, metricName, "Required metric %s is missing", metricName)
	}

	// Then validate all present metrics
	for n, v := range metrics {
		validator, ok := requiredMetrics[n]
		if !ok {
			// Unknown metric - just log it but don't fail the test
			t.Logf("Unknown metric %s with value %v - this is likely a new Anthropic metric", n, v)
			continue
		}
		assert.True(validator(v), "metric %s is not valid (value: %v)", n, v)
	}

	// a crude check to make sure all json is parsed
	assert.NotNil(span.Metadata())
	assert.NotNil(span.Input())
	assert.NotNil(span.Output())
}
