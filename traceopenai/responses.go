package traceopenai

//  this file parses the responses API.

import (
	"context"
	"encoding/json"

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

func (*v1ResponsesTracer) startSpanFromRequest(ctx context.Context, req requestData) (context.Context, trace.Span, error) {
	ctx, span := tracer().Start(ctx, "openai.responses.create")

	var responseRequest v1ResponsesPostRequest
	err := json.Unmarshal(req.body, &responseRequest)
	if err != nil {
		return ctx, nil, err
	}

	attrs := []attribute.KeyValue{
		attribute.String("provider", "openai"),
		attribute.String("model", responseRequest.Model),
		attribute.String("input", responseRequest.Input),
	}

	if responseRequest.Instructions != nil {
		attrs = append(attrs, attribute.String("instructions", *responseRequest.Instructions))
	}

	if responseRequest.User != nil {
		attrs = append(attrs, attribute.String("user", *responseRequest.User))
	}

	if responseRequest.Temperature != nil {
		attrs = append(attrs, attribute.Float64("temperature", *responseRequest.Temperature))
	}

	if responseRequest.TopP != nil {
		attrs = append(attrs, attribute.Float64("top_p", *responseRequest.TopP))
	}

	if responseRequest.ParallelToolCalls != nil {
		attrs = append(attrs, attribute.Bool("parallel_tool_calls", *responseRequest.ParallelToolCalls))
	}

	if responseRequest.Store != nil {
		attrs = append(attrs, attribute.Bool("store", *responseRequest.Store))
	}

	if responseRequest.Truncation != nil {
		attrs = append(attrs, attribute.String("truncation", *responseRequest.Truncation))
	}

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

// v1ResponsesPostRequest is the request body for the openai v1/responses POST endpoint.
type v1ResponsesPostRequest struct {
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
	ToolChoice         *string           `json:"tool_choice,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	Truncation         *string           `json:"truncation,omitempty"`
	User               *string           `json:"user,omitempty"`
	// FIXME[matt]
	// Tools              []tool            `json:"tools,omitempty"`
	// Text               *textConfig       `json:"text,omitempty"`
	// Reasoning          *reasoningConfig  `json:"reasoning,omitempty"`
}
