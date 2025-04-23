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

	// Start with basic attributes
	attrs := []attribute.KeyValue{
		attribute.String("provider", "openai"),
	}

	// Parse as a general map
	var requestMap map[string]interface{}
	if err := json.Unmarshal(req.body, &requestMap); err == nil {
		// Extract basic fields from the map
		if model, ok := requestMap["model"].(string); ok {
			attrs = append(attrs, attribute.String("model", model))
		}

		// Handle input which could be a string or complex type
		if input, ok := requestMap["input"].(string); ok {
			attrs = append(attrs, attribute.String("input", input))
		}

		// Check for other common fields
		if instructions, ok := requestMap["instructions"].(string); ok {
			attrs = append(attrs, attribute.String("instructions", instructions))
		}
		
		if user, ok := requestMap["user"].(string); ok {
			attrs = append(attrs, attribute.String("user", user))
		}
		
		if temperature, ok := requestMap["temperature"].(float64); ok {
			attrs = append(attrs, attribute.Float64("temperature", temperature))
		}
		
		if topP, ok := requestMap["top_p"].(float64); ok {
			attrs = append(attrs, attribute.Float64("top_p", topP))
		}
		
		if parallelToolCalls, ok := requestMap["parallel_tool_calls"].(bool); ok {
			attrs = append(attrs, attribute.Bool("parallel_tool_calls", parallelToolCalls))
		}
		
		if store, ok := requestMap["store"].(bool); ok {
			attrs = append(attrs, attribute.Bool("store", store))
		}
		
		if truncation, ok := requestMap["truncation"].(string); ok {
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
	
	// Add token usage metrics if present
	if response.JSON.Usage.IsPresent() {
		if response.Usage.JSON.InputTokens.IsPresent() {
			attrs = append(attrs, attribute.Int64("usage.input_tokens", response.Usage.InputTokens))
		}
		if response.Usage.JSON.OutputTokens.IsPresent() {
			attrs = append(attrs, attribute.Int64("usage.output_tokens", response.Usage.OutputTokens))
		}
		if response.Usage.JSON.TotalTokens.IsPresent() {
			attrs = append(attrs, attribute.Int64("usage.total_tokens", response.Usage.TotalTokens))
		}
		
		// Add detailed token metrics if present
		if response.Usage.JSON.InputTokensDetails.IsPresent() {
			// Extract any input token details fields that are present
			if response.Usage.InputTokensDetails.JSON.CachedTokens.IsPresent() {
				attrs = append(attrs, attribute.Int64("usage.input_tokens_details.cached_tokens", response.Usage.InputTokensDetails.CachedTokens))
			}
		}
		
		if response.Usage.JSON.OutputTokensDetails.IsPresent() {
			// Extract any output token details fields that are present
			if response.Usage.OutputTokensDetails.JSON.ReasoningTokens.IsPresent() {
				attrs = append(attrs, attribute.Int64("usage.output_tokens_details.reasoning_tokens", response.Usage.OutputTokensDetails.ReasoningTokens))
			}
		}
	}

	span.SetAttributes(attrs...)
	return nil
}

// We're using a generic map-based approach to parse OpenAI requests
