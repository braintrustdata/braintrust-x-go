package traceopenai

//  this file parses the responses API.

import (
	"context"
	"encoding/json"
	"time"

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
		{"input", "struct"},
		{"output", "struct"},
		{"service_tier", "string"},
		{"temperature", "float64"},
		{"top_p", "float64"},
		{"parallel_tool_calls", "bool"},
		{"store", "bool"},
		{"tools", "struct"},
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ctx, span, err
	}

	// keep a list of structs that must JSON encoded (because otel doesn't support structs)
	structs := make(map[string]interface{})

	// handle simple fields here.
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
			case "struct":
				structs[field.name] = value
			}
		}
	}

	if 0 < len(structs) {
		// otel doesn't support structs, so we need to marshal them to JSON and submit them
		// as string attributes.
		sb, err := json.Marshal(structs)
		if err != nil {
			return ctx, span, err
		}
		span.SetAttributes(attribute.String("attributes.json.input", string(sb)))
	}

	return ctx, span, nil
}

func (*v1ResponsesTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	//fmt.Println("body", string(body))

	var raw map[string]interface{}
	err := json.Unmarshal(body, &raw)
	if err != nil {
		return err
	}

	attrs := []attribute.KeyValue{}

	/// process usage tokens
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		for _, k := range []string{"input_tokens", "output_tokens", "total_tokens"} {
			if v, ok := usage[k].(float64); ok {
				attrs = append(attrs, attribute.Int64("usage."+k, int64(v)))
			}
		}
		for _, d := range []string{"input_tokens_details", "output_tokens_details"} {
			if details, ok := usage[d].(map[string]interface{}); ok {
				for k, v := range details {
					if c, ok := v.(float64); ok {
						attrs = append(attrs, attribute.Int64(d+"."+k, int64(c)))
					}
				}
			}
		}
	}

	// Handle basic string fields
	for _, k := range []string{"id", "model", "object"} {
		if v, ok := raw[k].(string); ok {
			attrs = append(attrs, attribute.String(k, v))
		}
	}

	structs := make(map[string]interface{})
	for _, k := range []string{"output"} {
		if v, ok := raw[k]; ok {
			structs[k] = v
		}
	}

	if 0 < len(structs) {
		sb, err := json.Marshal(structs)
		if err != nil {
			return err
		}
		attrs = append(attrs, attribute.String("attributes.json.output", string(sb)))
	}

	// Handle metadata if present
	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		for key, value := range metadata {
			if strValue, ok := value.(string); ok {
				attrs = append(attrs, attribute.String("metadata."+key, strValue))
			}
		}
	}

	span.SetAttributes(attrs...)
	return nil
}

var _ httpTracer = &v1ResponsesTracer{}
