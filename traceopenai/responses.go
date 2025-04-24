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
	ctx, span := tracer().Start(
		ctx,
		"openai.responses.create",
		trace.WithTimestamp(t),
		trace.WithAttributes(attribute.String("provider", "openai")),
	)

	// handle simple fields here.
	fields := []struct{ name, kind string }{
		{"model", "string"},
		{"instructions", "string"},
		{"user", "string"},
		{"truncation", "string"},
		{"input", "string"},
		{"temperature", "float64"},
		{"top_p", "float64"},
		{"parallel_tool_calls", "bool"},
		{"store", "bool"},
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ctx, span, err
	}

	for _, field := range fields {
		if value, exists := raw[field.name]; exists {
			switch field.kind {
			case "string":
				if v, ok := value.(string); ok {
					span.SetAttributes(attribute.String(field.name, v))
				}
			case "float64":
				if v, ok := value.(float64); ok {
					span.SetAttributes(attribute.Float64(field.name, v))
				}
			case "bool":
				if v, ok := value.(bool); ok {
					span.SetAttributes(attribute.Bool(field.name, v))
				}
			}
		}
	}

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
