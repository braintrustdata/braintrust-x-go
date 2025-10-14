// Package tracegenai provides OpenTelemetry tracing for Google Gemini API calls.
//
// First, set up tracing with Quickstart (requires BRAINTRUST_API_KEY environment variable):
//
//	// export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
// Then create your Gemini client with tracing:
//
//	client, err := genai.NewClient(ctx, &genai.ClientConfig{
//		HTTPClient: tracegenai.Client(),
//		APIKey:     apiKey,
//		Backend:    genai.BackendGeminiAPI,
//	})
//
//	// Your Gemini calls will now be automatically traced
//	resp, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-exp",
//		genai.Text("Hello!"), nil)
package tracegenai

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/internal"
)

// tracer returns the shared braintrust tracer.
func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

// Client returns a new http.Client configured with tracing middleware.
// This is equivalent to WrapClient(nil), which wraps the default HTTP transport.
func Client() *http.Client {
	return WrapClient(nil)
}

// WrapClient wraps an existing http.Client with tracing middleware.
// If client is nil, a new client with the default transport is created.
func WrapClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	// Get the existing transport or use default
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Wrap with our tracing RoundTripper
	client.Transport = newRoundTripper(transport)
	return client
}

// roundTripper wraps an http.RoundTripper with OpenTelemetry tracing.
type roundTripper struct {
	base http.RoundTripper
}

// newRoundTripper creates a new tracing RoundTripper that wraps the base transport.
func newRoundTripper(base http.RoundTripper) http.RoundTripper {
	return &roundTripper{base: base}
}

// RoundTrip implements http.RoundTripper by intercepting requests and responses.
func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Use the internal middleware infrastructure
	middleware := internal.Middleware(genaiRouter)

	// Create a NextMiddleware function that calls the base transport
	next := func(r *http.Request) (*http.Response, error) {
		return rt.base.RoundTrip(r)
	}

	return middleware(req, next)
}

// genaiRouter maps Gemini API paths to their corresponding tracers.
func genaiRouter(path string) internal.MiddlewareTracer {
	// Match both Gemini API and Vertex AI paths
	// Gemini API: /v1beta/models/{model}/generateContent
	// Vertex AI: /v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent
	if containsGenerateContent(path) {
		model := extractModelFromPath(path)
		return newGenerateContentTracer(model)
	}
	return nil
}

// containsGenerateContent checks if the path is for a generateContent endpoint
func containsGenerateContent(path string) bool {
	return strings.Contains(path, "/generateContent") ||
		strings.Contains(path, ":generateContent")
}

// extractModelFromPath extracts the model name from the URL path
// Gemini API: /v1beta/models/{model}/generateContent or /v1beta/models/{model}:generateContent
// Vertex AI: /v1/projects/{project}/locations/{location}/publishers/google/models/{model}:generateContent
func extractModelFromPath(path string) string {
	// Split by /models/ to get the part after it
	parts := strings.Split(path, "/models/")
	if len(parts) < 2 {
		return ""
	}

	// Get the model part (after /models/)
	modelPart := parts[1]

	// Remove :generateContent or /generateContent suffix
	modelPart = strings.TrimSuffix(modelPart, ":generateContent")
	modelPart = strings.TrimSuffix(modelPart, "/generateContent")

	return modelPart
}
