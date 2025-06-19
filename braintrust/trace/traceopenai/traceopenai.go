// Package traceopenai provides OpenTelemetry middleware for tracing OpenAI API calls.
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
// Then add the middleware to your OpenAI client:
//
//	client := openai.NewClient(
//		option.WithMiddleware(traceopenai.Middleware),
//	)
//
//	// Your OpenAI calls will now be automatically traced
//	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
//		Model: openai.F(openai.ChatModelGPT4),
//		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
//			openai.UserMessage("Hello!"),
//		}),
//	})
package traceopenai

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

// Middleware adds OpenTelemetry tracing to OpenAI client requests.
// Ensure OpenTelemetry is properly configured before using this middleware.
var Middleware = internal.Middleware(openaiRouter)

// openaiRouter maps OpenAI API paths to their corresponding tracers.
func openaiRouter(path string) internal.MiddlewareTracer {
	switch path {
	case "/v1/responses":
		return newResponsesTracer()
	case "/v1/chat/completions":
		return newChatCompletionsTracer()
	default:
		return internal.NewNoopTracer()
	}
}

// Ensure our tracers implement the shared interface
var _ internal.MiddlewareTracer = &responsesTracer{}
var _ internal.MiddlewareTracer = &chatCompletionsTracer{}
