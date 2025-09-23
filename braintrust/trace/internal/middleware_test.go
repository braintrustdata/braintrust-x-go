package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMiddlewareWithNoopTracer tests that the middleware works correctly
// when falling back to NoopTracer for unsupported endpoints.
func TestMiddlewareWithNoopTracer(t *testing.T) {
	// Test request body
	requestBody := `{"items": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello!"}]}]}`

	// Create a test server that echoes back the request body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		// Verify the body matches what we sent
		assert.Equal(t, requestBody, string(body))

		// Echo back the request body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create a router that returns NoopTracer for unsupported paths
	router := func(path string) MiddlewareTracer {
		// Simulate unsupported endpoint (like /v1/conversations)
		if strings.HasSuffix(path, "/v1/conversations") {
			return NewNoopTracer()
		}
		return NewNoopTracer() // Default to noop for this test
	}

	// Create middleware with our router
	middleware := Middleware(router) //nolint:bodyclose // Test middleware, response is properly closed

	// Create a test request
	req, err := http.NewRequest("POST", server.URL+"/v1/conversations", strings.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create a next function that makes the actual HTTP request
	next := func(r *http.Request) (*http.Response, error) {
		client := &http.Client{}
		return client.Do(r)
	}

	// Execute the middleware
	resp, err := middleware(req, next)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()

	// Verify we got a successful response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the response body matches what we sent
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, requestBody, string(respBody))
}

// TestMiddlewareWithNilBody tests that the middleware handles nil request bodies correctly
func TestMiddlewareWithNilBody(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify body is nil or empty
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Empty(t, body)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"success": true}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create a router that returns NoopTracer
	router := func(path string) MiddlewareTracer {
		return NewNoopTracer()
	}

	// Create middleware
	middleware := Middleware(router) //nolint:bodyclose // Test middleware, response is properly closed

	// Create a test request with nil body (like a GET request)
	req, err := http.NewRequest("GET", server.URL+"/v1/conversations", nil)
	require.NoError(t, err)

	// Create next function
	next := func(r *http.Request) (*http.Response, error) {
		client := &http.Client{}
		return client.Do(r)
	}

	// Execute the middleware - this should not panic
	resp, err := middleware(req, next)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()

	// Verify successful response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestNoopTracerStartSpan tests that NoopTracer properly consumes the reader
func TestNoopTracerStartSpan(t *testing.T) {
	tracer := NewNoopTracer()

	// Create a test reader with some data
	testData := "test request body data"
	reader := strings.NewReader(testData)

	// Create a buffer to simulate TeeReader behavior
	var buf bytes.Buffer
	teeReader := io.TeeReader(reader, &buf)

	// Call StartSpan - this should consume the entire teeReader
	ctx := context.Background()
	newCtx, span, err := tracer.StartSpan(ctx, time.Now(), teeReader)

	// Verify no error
	require.NoError(t, err)
	assert.NotNil(t, span)
	assert.Equal(t, ctx, newCtx) // Context should be passed through

	// Verify the buffer was populated (meaning teeReader was consumed)
	assert.Equal(t, testData, buf.String())

	// Verify the span is non-recording
	assert.False(t, span.IsRecording())
}

// TestNoopTracerStartSpanWithNilReader tests NoopTracer with nil reader
func TestNoopTracerStartSpanWithNilReader(t *testing.T) {
	tracer := NewNoopTracer()

	ctx := context.Background()
	newCtx, span, err := tracer.StartSpan(ctx, time.Now(), nil)

	// Should not panic or error
	require.NoError(t, err)
	assert.NotNil(t, span)
	assert.Equal(t, ctx, newCtx)
	assert.False(t, span.IsRecording())
}
