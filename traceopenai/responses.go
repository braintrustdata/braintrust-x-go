package traceopenai

//  this file parses the responses API.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openai/openai-go/responses"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// v1ResponsesTracer is a tracer for the openai v1/responses POST endpoint.
// See docs here: https://platform.openai.com/docs/api-reference/responses/create
type v1ResponsesTracer struct{}

func NewV1ResponsesTracer() *v1ResponsesTracer {
	return &v1ResponsesTracer{}
}

func (*v1ResponsesTracer) startSpanFromRequest(ctx context.Context, t time.Time, body []byte) (context.Context, trace.Span, error) {
	ctx, span := tracer().Start(ctx, "openai.responses.create", trace.WithTimestamp(t))

	// Start with basic attributes
	attrs := []attribute.KeyValue{
		attribute.String("provider", "openai"),
	}

	// Parse as a general map
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		// Extract basic fields from the map
		if model, ok := raw["model"].(string); ok {
			span.SetAttributes(attribute.String("model", model))
		}

		// Handle input which could be a string or complex type
		if input, ok := raw["input"].(string); ok {
			attrs = append(attrs, attribute.String("input", input))
		}

		// Check for other common fields
		if instructions, ok := raw["instructions"].(string); ok {
			attrs = append(attrs, attribute.String("instructions", instructions))
		}

		if user, ok := raw["user"].(string); ok {
			attrs = append(attrs, attribute.String("user", user))
		}

		if temperature, ok := raw["temperature"].(float64); ok {
			attrs = append(attrs, attribute.Float64("temperature", temperature))
		}

		if topP, ok := raw["top_p"].(float64); ok {
			attrs = append(attrs, attribute.Float64("top_p", topP))
		}

		if parallelToolCalls, ok := raw["parallel_tool_calls"].(bool); ok {
			attrs = append(attrs, attribute.Bool("parallel_tool_calls", parallelToolCalls))
		}

		if store, ok := raw["store"].(bool); ok {
			attrs = append(attrs, attribute.Bool("store", store))
		}

		if truncation, ok := raw["truncation"].(string); ok {
			attrs = append(attrs, attribute.String("truncation", truncation))
		}
	}
	// If parsing fails, we just continue with basic attributes

	span.SetAttributes(attrs...)
	return ctx, span, nil
}

func (*v1ResponsesTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	var response responses.Response
	err := json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	attrs := []attribute.KeyValue{
		attribute.String("id", response.ID),
		attribute.String("model", string(response.Model)),
		attribute.String("object", string(response.Object)),
	}

	// Add the output_text directly using the helper method
	outputText := response.OutputText()
	if outputText != "" {
		attrs = append(attrs, attribute.String("output", outputText))
	}

	if response.JSON.Metadata.IsPresent() {
		for key, value := range response.Metadata {
			attrs = append(attrs, attribute.String("metadata."+key, value))
		}
	}

	span.SetAttributes(attrs...)
	return nil
}

var _ requestTracer = &v1ResponsesTracer{}
