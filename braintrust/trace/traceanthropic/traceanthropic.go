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
	switch path {
	case "/v1/messages":
		return newMessagesTracer()
	default:
		return internal.NewNoopTracer()
	}
}

// Ensure our tracers implement the shared interface
var _ internal.MiddlewareTracer = &messagesTracer{}
