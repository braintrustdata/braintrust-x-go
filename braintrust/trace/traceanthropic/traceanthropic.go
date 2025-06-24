// Package traceanthropic provides OpenTelemetry middleware for tracing Anthropic API calls.
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
// Then add the middleware to your Anthropic client:
//
//	client := anthropic.NewClient(
//		option.WithMiddleware(traceanthropic.Middleware),
//	)
//
//	// Your Anthropic calls will now be automatically traced
//	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
//		Model: anthropic.ModelClaude3_7SonnetLatest,
//		Messages: []anthropic.MessageParam{
//			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello!")),
//		},
//		MaxTokens: 1024,
//	})
package traceanthropic

import (
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/trace/internal"
)

// NextMiddleware is re-exported from internal for backward compatibility.
type NextMiddleware = internal.NextMiddleware

// tracer returns the shared braintrust tracer.
func tracer() trace.Tracer {
	return internal.GetTracer()
}

// Middleware adds OpenTelemetry tracing to Anthropic client requests.
// Ensure OpenTelemetry is properly configured before using this middleware.
var Middleware = internal.Middleware(anthropicRouter)

// anthropicRouter maps Anthropic API paths to their corresponding tracers.
func anthropicRouter(path string) internal.MiddlewareTracer {
	if path == "/v1/messages" {
		return newMessagesTracer()
	}
	return internal.NewNoopTracer()

}

// parseUsageTokens parses the usage tokens from Anthropic API responses
// It handles Anthropic-specific fields including cache tokens
func parseUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	var inputTokens, cacheCreationTokens, cacheReadTokens int64

	// Single pass: process all tokens
	for k, v := range usage {
		if ok, i := internal.ToInt64(v); ok {
			switch k {
			case "input_tokens":
				inputTokens = i
			case "cache_creation_input_tokens":
				cacheCreationTokens = i
				metrics["cache_creation_input_tokens"] = i
			case "cache_read_input_tokens":
				cacheReadTokens = i
				metrics["cache_read_input_tokens"] = i
			case "output_tokens":
				metrics["completion_tokens"] = i
			case "total_tokens":
				metrics["tokens"] = i
			default:
				// Keep other fields as-is (future-proofing for new Anthropic fields)
				metrics[k] = i
			}
		}
	}

	// Calculate total prompt tokens (input + cache tokens)
	totalPromptTokens := inputTokens + cacheCreationTokens + cacheReadTokens
	if totalPromptTokens > 0 {
		metrics["prompt_tokens"] = totalPromptTokens
	}

	return metrics
}

// Ensure our tracers implement the shared interface
var _ internal.MiddlewareTracer = &messagesTracer{}
