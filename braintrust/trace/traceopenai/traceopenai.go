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
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/internal"
)

// NextMiddleware represents the next middleware to run in the OpenAI client middleware chain.
type NextMiddleware = internal.NextMiddleware

// tracer returns the shared braintrust tracer.
func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

// Middleware adds OpenTelemetry tracing to OpenAI client requests.
// Ensure OpenTelemetry is properly configured before using this middleware.
var Middleware = internal.Middleware(openaiRouter)

func openaiRouter(path string) internal.MiddlewareTracer {

	// we map suffix => tracer because some OpenAI compatible endpoints have a different BaseURL and
	// therefore a different path here. For example:
	// 	- OpenAI has /v1/chat/completions
	//  - OpenRouter has /api/v1/chat/completions.
	// See https://github.com/braintrustdata/braintrust-x-go/issues/36
	if strings.HasSuffix(path, "/v1/chat/completions") {
		return newChatCompletionsTracer()
	}

	if strings.HasSuffix(path, "/v1/responses") {
		return newResponsesTracer()
	}

	return nil
}

// parseUsageTokens parses the usage tokens from OpenAI API responses
// It handles different API formats using a unified approach
func parseUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	// Parse token metrics and translate names to be consistent
	for k, v := range usage {
		if strings.HasSuffix(k, "_tokens_details") {
			prefix := translateMetricPrefix(strings.TrimSuffix(k, "_tokens_details"))
			if details, ok := v.(map[string]interface{}); ok {
				for kd, vd := range details {
					if ok, i := internal.ToInt64(vd); ok {
						metrics[prefix+"_"+kd] = i
					}
				}
			}
		} else {
			if ok, i := internal.ToInt64(v); ok {
				switch k {
				case "input_tokens":
					metrics["prompt_tokens"] = i
				case "output_tokens":
					metrics["completion_tokens"] = i
				case "total_tokens":
					metrics["tokens"] = i
				default:
					// Keep other fields as-is (future-proofing for new OpenAI fields)
					metrics[k] = i
				}
			}
		}
	}

	return metrics
}

// translateMetricPrefix translates metric prefixes to be consistent between APIs
func translateMetricPrefix(prefix string) string {
	switch prefix {
	case "input":
		return "prompt"
	case "output":
		return "completion"
	default:
		return prefix
	}
}

// Ensure our tracers implement the shared interface
var _ internal.MiddlewareTracer = &responsesTracer{}
var _ internal.MiddlewareTracer = &chatCompletionsTracer{}
