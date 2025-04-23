package traceopenai

//  this file parses the responses API.

import (
	"context"
	"encoding/json"

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
	span.SetAttributes(attrs...)

	return ctx, span, nil
}

func (*v1ResponsesTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	var response v1ResponsesPostResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	attrs := []attribute.KeyValue{
		attribute.String("id", response.ID),
		attribute.String("model", response.Model),
		attribute.String("object", response.Object),
	}

	// Handle Output field which can be string or array
	if outputStr, ok := response.Output.(string); ok {
		attrs = append(attrs, attribute.String("output", outputStr))
	} else if outputArr, ok := response.Output.([]interface{}); ok {
		// Convert array to string representation
		outputBytes, err := json.Marshal(outputArr)
		if err != nil {
			return err
		}
		attrs = append(attrs, attribute.String("output", string(outputBytes)))
	}

	if response.Metadata != nil {
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
	ToolChoice         string            `json:"tool_choice,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	Truncation         *string           `json:"truncation,omitempty"`
	User               *string           `json:"user,omitempty"`
	// FIXME[matt]
	// Tools              []tool            `json:"tools,omitempty"`
	// Text               *textConfig       `json:"text,omitempty"`
	// Reasoning          *reasoningConfig  `json:"reasoning,omitempty"`
}

// v1ResponsesPostResponse is the response body for the openai v1/responses POST endpoint.
type v1ResponsesPostResponse struct {
	ID                 string            `json:"id"`
	Model              string            `json:"model"`
	Created            int               `json:"created"`
	Object             string            `json:"object"`
	Output             interface{}       `json:"output"`
	Usage              *Usage            `json:"usage,omitempty"`
	ServiceTier        *string           `json:"service_tier,omitempty"`
	PreviousResponseID *string           `json:"previous_response_id,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
