package tracegenai

// this file parses the generateContent API.

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/internal"
)

// generateContentTracer is a tracer for the Gemini generateContent endpoint.
type generateContentTracer struct {
	streaming bool
	metadata  map[string]any
}

func newGenerateContentTracer() *generateContentTracer {
	return &generateContentTracer{
		streaming: false,
		metadata: map[string]any{
			"provider": "gemini",
		},
	}
}

func (gt *generateContentTracer) StartSpan(ctx context.Context, t time.Time, request io.Reader) (context.Context, trace.Span, error) {
	ctx, span := tracer().Start(
		ctx,
		"genai.models.generateContent",
		trace.WithTimestamp(t),
	)

	var raw map[string]interface{}
	if err := json.NewDecoder(request).Decode(&raw); err != nil {
		return ctx, span, err
	}

	// Extract metadata fields from request
	metadataFields := []string{
		"model",
		"systemInstruction",
		"tools",
		"toolConfig",
		"safetySettings",
		"cachedContent",
	}

	for _, field := range metadataFields {
		if value, exists := raw[field]; exists {
			gt.metadata[field] = value
		}
	}

	// Handle generationConfig
	if genConfig, ok := raw["generationConfig"].(map[string]any); ok {
		configFields := []string{
			"temperature",
			"topP",
			"topK",
			"candidateCount",
			"maxOutputTokens",
			"stopSequences",
			"responseMimeType",
			"responseSchema",
		}
		for _, field := range configFields {
			if value, exists := genConfig[field]; exists {
				gt.metadata[field] = value
			}
		}
	}

	// Parse input contents into messages format
	if contents, ok := raw["contents"].([]any); ok {
		var msgs []any

		// Prepend system instruction if present
		if systemInst, ok := raw["systemInstruction"].(map[string]any); ok {
			if parts, ok := systemInst["parts"].([]any); ok {
				msgs = append(msgs, map[string]any{
					"role":    "system",
					"content": parts,
				})
			}
		}

		// Add content messages
		for _, content := range contents {
			if contentMap, ok := content.(map[string]any); ok {
				role := "user" // default role
				if r, ok := contentMap["role"].(string); ok {
					role = r
				}

				msg := map[string]any{
					"role":    role,
					"content": contentMap["parts"],
				}
				msgs = append(msgs, msg)
			}
		}

		if len(msgs) > 0 {
			if err := internal.SetJSONAttr(span, "braintrust.input_json", msgs); err != nil {
				return ctx, span, err
			}
		}
	}

	if err := internal.SetJSONAttr(span, "braintrust.metadata", gt.metadata); err != nil {
		return ctx, span, err
	}

	// Set span attributes to mark this as an LLM span
	spanAttrs := map[string]string{
		"type": "llm",
	}
	if err := internal.SetJSONAttr(span, "braintrust.span_attributes", spanAttrs); err != nil {
		return ctx, span, err
	}

	return ctx, span, nil
}

func (gt *generateContentTracer) TagSpan(span trace.Span, body io.Reader) error {
	// For now, handle non-streaming responses
	// Streaming will be handled separately
	return gt.parseResponse(span, body)
}

func (gt *generateContentTracer) parseResponse(span trace.Span, body io.Reader) error {
	var raw map[string]interface{}
	err := json.NewDecoder(body).Decode(&raw)
	if err != nil {
		return err
	}

	return gt.handleResponse(span, raw)
}

func (gt *generateContentTracer) handleResponse(span trace.Span, raw map[string]any) error {
	// Extract model version if present
	if modelVersion, ok := raw["modelVersion"].(string); ok {
		gt.metadata["model"] = modelVersion
	}

	// Update metadata
	if err := internal.SetJSONAttr(span, "braintrust.metadata", gt.metadata); err != nil {
		return err
	}

	// Parse candidates into output format
	if candidates, ok := raw["candidates"].([]any); ok && len(candidates) > 0 {
		var outputMsgs []any

		for _, candidate := range candidates {
			if candMap, ok := candidate.(map[string]any); ok {
				if content, ok := candMap["content"].(map[string]any); ok {
					role := "assistant" // default
					if r, ok := content["role"].(string); ok {
						role = r
					}

					msg := map[string]any{
						"role":    role,
						"content": content["parts"],
					}

					// Add finish reason if present
					if finishReason, ok := candMap["finishReason"].(string); ok {
						msg["finishReason"] = finishReason
					}

					outputMsgs = append(outputMsgs, msg)
				}
			}
		}

		if len(outputMsgs) > 0 {
			if err := internal.SetJSONAttr(span, "braintrust.output_json", outputMsgs); err != nil {
				return err
			}
		}
	}

	// Parse usage metadata (token counts)
	if usageMetadata, ok := raw["usageMetadata"].(map[string]any); ok {
		metrics := parseUsageTokens(usageMetadata)
		if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			return err
		}
	}

	return nil
}

// parseUsageTokens parses the usage tokens from Gemini API responses
func parseUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	for k, v := range usage {
		if ok, i := internal.ToInt64(v); ok {
			switch k {
			case "promptTokenCount":
				metrics["prompt_tokens"] = i
			case "candidatesTokenCount":
				metrics["completion_tokens"] = i
			case "totalTokenCount":
				metrics["tokens"] = i
			case "cachedContentTokenCount":
				metrics["prompt_cached_tokens"] = i
			default:
				// Keep other fields as-is for future-proofing
				// Convert camelCase to snake_case for consistency
				snakeKey := camelToSnake(k)
				metrics[snakeKey] = i
			}
		}
	}

	return metrics
}

// camelToSnake converts camelCase to snake_case
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// Ensure our tracer implements the shared interface
var _ internal.MiddlewareTracer = &generateContentTracer{}
