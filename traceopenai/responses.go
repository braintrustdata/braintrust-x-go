package traceopenai

//  this file parses the responses API.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ResponsesTracer is a tracer for the openai v1/responses POST endpoint.
// See docs here: https://platform.openai.com/docs/api-reference/responses/create
type ResponsesTracer struct {
	streaming bool
}

func NewResponsesTracer() *ResponsesTracer {
	return &ResponsesTracer{streaming: false}
}

func (rt *ResponsesTracer) startSpanFromRequest(ctx context.Context, t time.Time, body []byte) (context.Context, trace.Span, error) {
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
		{"max_output_tokens", "int"},
		{"timeout", "float64"},
		{"parallel_tool_calls", "bool"},
		{"store", "bool"},
		{"stream", "bool"},
		{"tools", "struct"},
		{"tool_choice", "struct"},
		{"seed", "int"},
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
			case "int":
				if v, ok := value.(float64); ok {
					span.SetAttributes(attribute.Int64(field.name, int64(v)))
				}
			case "bool":
				if v, ok := value.(bool); ok {
					span.SetAttributes(attribute.Bool(field.name, v))
					if field.name == "stream" {
						rt.streaming = v
					}
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
		span.SetAttributes(attribute.String("attributes.json.request", string(sb)))
	}

	return ctx, span, nil
}

func (rt *ResponsesTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	if rt.streaming {
		return parseStreamingResponse(span, bytes.NewReader(body))
	} else {
		return parseResponse(span, body)
	}
}

func parseStreamingResponse(span trace.Span, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		line = strings.TrimPrefix(line, "data: ")
		var envelope map[string]any
		err := json.Unmarshal([]byte(line), &envelope)
		if err != nil {
			return err
		}

		if msgType, ok := envelope["type"].(string); ok {
			if msgType == "response.completed" {
				if msg, ok := envelope["response"].(map[string]any); ok {
					if err := handleResponseCompletedMessage(span, msg); err != nil {
						return err
					}
				}
			}
		}
	}

	return scanner.Err()
}

func parseResponse(span trace.Span, body []byte) error {
	var raw map[string]interface{}
	err := json.Unmarshal(body, &raw)
	if err != nil {
		return err
	}

	return handleResponseCompletedMessage(span, raw)
}

func handleResponseCompletedMessage(span trace.Span, rawMsg map[string]any) error {

	attrs := []attribute.KeyValue{}

	fields := []struct{ name, kind string }{
		{"id", "string"},
		{"model", "string"},
		{"object", "string"},
		{"system_fingerprint", "string"},
		{"completion_tokens", "int"},
		{"output", "struct"},
		{"tool_calls", "struct"},
		{"prompt_filter_results", "struct"},
		{"usage", "usage"},
		{"metadata", "metadata"},
	}

	structs := make(map[string]interface{})
	for _, field := range fields {
		if v, ok := rawMsg[field.name]; ok {
			switch field.kind {
			case "string":
				attrs = append(attrs, attribute.String(field.name, v.(string)))
			case "int":
				attrs = append(attrs, attribute.Int64(field.name, v.(int64)))
			case "struct":
				structs[field.name] = v
			case "usage":
				if usage, ok := v.(map[string]interface{}); ok {
					parseUsageTokens(usage, span)
				}
			case "metadata":
				if metadata, ok := v.(map[string]interface{}); ok {
					for key, value := range metadata {
						attrs = append(attrs, attribute.String("metadata."+key, value.(string)))
					}
				}
			}
		}
	}

	if 0 < len(structs) {
		sb, err := json.Marshal(structs)
		if err != nil {
			return err
		}
		attrs = append(attrs, attribute.String("attributes.json.response", string(sb)))
	}

	span.SetAttributes(attrs...)

	return nil
}

// parseUsageTokens parses the usage tokens from the response and adds them to the span.
func parseUsageTokens(usage map[string]interface{}, span trace.Span) {
	attrs := []attribute.KeyValue{}
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

	span.SetAttributes(attrs...)
}

var _ httpTracer = &ResponsesTracer{}
