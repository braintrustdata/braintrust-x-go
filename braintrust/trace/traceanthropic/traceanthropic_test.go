package traceanthropic

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/braintrust/braintrust-x-go/braintrust/trace/internal"
)

func TestMiddleware(t *testing.T) {
	// Create a test request with a Messages API call
	requestBody := `{
		"model": "claude-3-7-sonnet-20250219",
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
		"model": "claude-3-7-sonnet-20250219",
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
		"model": "claude-3-7-sonnet-20250219",
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
	assert.Equal(t, "claude-3-7-sonnet-20250219", tracer.metadata["model"])
	assert.Equal(t, float64(1024), tracer.metadata["max_tokens"])
	assert.Equal(t, false, tracer.metadata["stream"])
	assert.False(t, tracer.streaming)
}

func TestMessagesTracerStreaming(t *testing.T) {
	tracer := newMessagesTracer()

	// Test streaming request
	requestBody := `{
		"model": "claude-3-7-sonnet-20250219",
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

	metrics := internal.ParseUsageTokens(usage)

	assert.Equal(t, int64(12), metrics["prompt_tokens"])
	assert.Equal(t, int64(9), metrics["completion_tokens"])
}

func TestBufferedReader(t *testing.T) {
	content := "test content"
	src := io.NopCloser(strings.NewReader(content))

	var capturedContent string
	onDone := func(r io.Reader) {
		data, _ := io.ReadAll(r)
		capturedContent = string(data)
	}

	reader := internal.NewBufferedReader(src, onDone)

	// Read the content
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))

	// Close to trigger onDone
	err = reader.Close()
	require.NoError(t, err)

	// Verify onDone was called with the correct content
	assert.Equal(t, content, capturedContent)
}

func TestNoopTracer(t *testing.T) {
	tracer := internal.NewNoopTracer()
	assert.NotNil(t, tracer)

	ctx := t.Context()
	start := time.Now()
	reader := &bytes.Buffer{}

	newCtx, span, err := tracer.StartSpan(ctx, start, reader)
	require.NoError(t, err)
	require.NotNil(t, span)
	require.NotNil(t, newCtx)

	err = tracer.TagSpan(span, reader)
	require.NoError(t, err)
}
