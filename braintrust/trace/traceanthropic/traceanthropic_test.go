package traceanthropic

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
