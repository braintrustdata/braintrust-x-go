package traceopenai

//  this file parses the responses API.

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type v1ResponseRequest struct {
	Model              string            `json:"model,omitempty"`
	Input              string            `json:"input,omitempty"`
	Include            []string          `json:"include,omitempty"`
	Instructions       *string           `json:"instructions,omitempty"`
	MaxOutputTokens    *int              `json:"max_output_tokens,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	ParallelToolCalls  *bool             `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID *string           `json:"previous_response_id,omitempty"`
	ServiceTier        *string           `json:"service_tier,omitempty"`
	Store              *bool             `json:"store,omitempty"`
	Stream             *bool             `json:"stream,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	ToolChoice         string            `json:"tool_choice,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	Truncation         *string           `json:"truncation,omitempty"`
	User               *string           `json:"user,omitempty"`
	// FIXME[matt]
	// Tools              []tool            `json:"tools,omitempty"`
	// Text               *textConfig       `json:"text,omitempty"`
	// Reasoning          *reasoningConfig  `json:"reasoning,omitempty"`
}

type v1ResponsesTracer struct{}

func NewV1ResponsesTracer() *v1ResponsesTracer {
	return &v1ResponsesTracer{}
}

func (*v1ResponsesTracer) startSpanFromRequest(ctx context.Context, req requestData) (trace.Span, error) {
	// post https://api.openai.com/v1/responses
	// handles https://platform.openai.com/docs/api-reference/responses/create
	_, span := tracer.Start(ctx, "openai.chat.completion")

	var responseRequest v1ResponseRequest
	err := json.Unmarshal(req.body, &responseRequest)
	if err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{
		attribute.String("provider", "openai"),
		attribute.String("model", responseRequest.Model),
		attribute.String("input", responseRequest.Input),
	}
	span.SetAttributes(attrs...)

	return span, nil
}

func (*v1ResponsesTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	// var responseResponse v1ResponseResponse
	// err := json.Unmarshal(body, &responseResponse)
	// if err != nil {
	// 	return err
	// }

	// span.SetAttributes(attribute.String("response", string(body)))

	return nil
}
