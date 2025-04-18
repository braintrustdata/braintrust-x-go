package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/openai/openai-go/option"
)

// LoggingMiddleware implements openai.TransportMiddleware to log all request/response data
func LoggingMiddleware(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	// Log request
	fmt.Printf("OpenAI Request: %s %s\n", req.Method, req.URL.String())

	fmt.Printf("OpenAI Request Context: %v\n", req.Context())

	// Preserve the original body for reading
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Printf("Error reading request body: %v\n", err)
			return next(req) // Continue with request even if we can't read it
		}

		// Log request body
		fmt.Printf("Request Body: %s\n", string(bodyBytes))

		// Pretty print if it's JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
			fmt.Printf("Request JSON:\n%s\n", prettyJSON.String())
		}

		// Replace the body with a new reader containing the same data
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Forward the request to the next handler
	resp, err := next(req)

	// Log response
	if err != nil {
		fmt.Printf("OpenAI Response Error: %v\n", err)
		return resp, err
	}

	fmt.Printf("OpenAI Response: %s %s -> %d %s\n",
		req.Method, req.URL.String(), resp.StatusCode, resp.Status)

	// Read and preserve the response body
	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading response body: %v\n", err)
			return resp, nil // Continue with response even if we can't read it
		}

		// Log response body
		fmt.Printf("Response Body: %s\n", string(bodyBytes))

		// Pretty print if it's JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
			fmt.Printf("Response JSON:\n%s\n", prettyJSON.String())
		}

		// Replace the body with a new reader containing the same data
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return resp, err
}
