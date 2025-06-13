package traceopenai

// this file parses the chat completions API.

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// chatCompletionsTracer is a tracer for the openai v1/chat/completions POST endpoint.
// See docs here: https://platform.openai.com/docs/api-reference/chat/create
type chatCompletionsTracer struct {
	streaming bool
	metadata  map[string]any
}

func newChatCompletionsTracer() *chatCompletionsTracer {
	return &chatCompletionsTracer{
		streaming: false,
		metadata: map[string]any{
			"provider": "openai",
			"endpoint": "/v1/chat/completions",
		},
	}
}

func (ct *chatCompletionsTracer) StartSpan(ctx context.Context, t time.Time, request io.Reader) (context.Context, trace.Span, error) {
	ctx, span := tracer().Start(
		ctx,
		"openai.chat.completions.create",
		trace.WithTimestamp(t),
	)

	var raw map[string]interface{}
	if err := json.NewDecoder(request).Decode(&raw); err != nil {
		return ctx, span, err
	}

	metadataFields := []string{
		"model",
		"frequency_penalty",
		"logit_bias",
		"logprobs",
		"max_tokens",
		"n",
		"presence_penalty",
		"response_format",
		"seed",
		"service_tier",
		"stop",
		"stream",
		"stream_options",
		"temperature",
		"top_p",
		"top_logprobs",
		"tools",
		"tool_choice",
		"parallel_tool_calls",
		"user",
		"functions",
		"function_call",
	}

	// handle simple fields here.
	for _, field := range metadataFields {
		if value, exists := raw[field]; exists {
			ct.metadata[field] = value
			// keep track of streaming requests so we can parse the streaming response later.
			if field == "stream" {
				if value, ok := value.(bool); ok {
					ct.streaming = value
				}
			}
		}
	}

	if messages, ok := raw["messages"]; ok {
		b, err := json.Marshal(messages)
		if err != nil {
			return ctx, span, err
		}
		span.SetAttributes(attribute.String("braintrust.input", string(b)))
	}

	b, err := json.Marshal(ct.metadata)
	if err != nil {
		return ctx, span, err
	}
	span.SetAttributes(attribute.String("braintrust.metadata", string(b)))

	return ctx, span, nil
}

func (ct *chatCompletionsTracer) TagSpan(span trace.Span, body io.Reader) error {
	if ct.streaming {
		return ct.parseStreamingResponse(span, body)
	} else {
		return ct.parseResponse(span, body)
	}
}

func (ct *chatCompletionsTracer) parseStreamingResponse(span trace.Span, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	var allChoices []map[string]any

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		line = strings.TrimPrefix(line, "data: ")
		if line == "[DONE]" {
			// End of stream
			break
		}

		var chunk map[string]any
		err := json.Unmarshal([]byte(line), &chunk)
		if err != nil {
			return err
		}

		// Aggregate choices from streaming chunks
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if allChoices == nil {
				allChoices = make([]map[string]any, len(choices))
				for i := range allChoices {
					allChoices[i] = map[string]any{
						"index": i,
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
						},
					}
				}
			}

			for _, choice := range choices {
				if choiceMap, ok := choice.(map[string]any); ok {
					idx := int(choiceMap["index"].(float64))
					if idx < len(allChoices) {
						if delta, ok := choiceMap["delta"].(map[string]any); ok {
							if content, ok := delta["content"].(string); ok {
								currentContent := allChoices[idx]["message"].(map[string]any)["content"].(string)
								allChoices[idx]["message"].(map[string]any)["content"] = currentContent + content
							}
							if role, ok := delta["role"].(string); ok {
								allChoices[idx]["message"].(map[string]any)["role"] = role
							}
							if toolCalls, ok := delta["tool_calls"]; ok {
								allChoices[idx]["message"].(map[string]any)["tool_calls"] = toolCalls
							}
						}
						if finishReason, ok := choiceMap["finish_reason"]; ok && finishReason != nil {
							allChoices[idx]["finish_reason"] = finishReason
						}
					}
				}
			}
		}

		// Handle usage in streaming response (if stream_options.include_usage is true)
		if usage, ok := chunk["usage"]; ok {
			ct.metadata["usage"] = usage
		}
	}

	// Set the aggregated output
	if allChoices != nil {
		if err := setJSONAttr(span, "braintrust.output", allChoices); err != nil {
			return err
		}
	}

	// Handle usage metrics
	if usage, ok := ct.metadata["usage"].(map[string]any); ok {
		metrics := parseChatUsageTokens(usage)
		if err := setJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (ct *chatCompletionsTracer) parseResponse(span trace.Span, body io.Reader) error {
	var raw map[string]interface{}
	err := json.NewDecoder(body).Decode(&raw)
	if err != nil {
		return err
	}

	return ct.handleChatCompletionResponse(span, raw)
}

func (ct *chatCompletionsTracer) handleChatCompletionResponse(span trace.Span, rawMsg map[string]any) error {
	metadataFields := []string{
		"id",
		"object",
		"created",
		"system_fingerprint",
		"service_tier",
	}

	for _, field := range metadataFields {
		if v, ok := rawMsg[field]; ok {
			ct.metadata[field] = v
		}
	}

	if err := setJSONAttr(span, "braintrust.metadata", ct.metadata); err != nil {
		return err
	}

	if usage, ok := rawMsg["usage"].(map[string]any); ok {
		metrics := parseChatUsageTokens(usage)
		if err := setJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			return err
		}
	}

	if choices, ok := rawMsg["choices"]; ok {
		if err := setJSONAttr(span, "braintrust.output", choices); err != nil {
			return err
		}
	}

	return nil
}

// parseChatUsageTokens parses the usage tokens from the chat completions response
func parseChatUsageTokens(usage map[string]interface{}) map[string]int64 {
	metrics := make(map[string]int64)

	if usage == nil {
		return metrics
	}

	// Direct token fields for chat completions
	directTokenFields := []string{"completion_tokens", "prompt_tokens", "total_tokens"}

	for _, field := range directTokenFields {
		if v, exists := usage[field]; exists {
			if ok, i := toInt64(v); ok {
				translatedKey := translateMetricKey(field)
				metrics[translatedKey] = i
			}
		}
	}

	// Handle detailed token breakdowns if present
	if details, ok := usage["completion_tokens_details"].(map[string]interface{}); ok {
		for k, v := range details {
			if ok, i := toInt64(v); ok {
				metrics["completion_"+k] = i
			}
		}
	}

	if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		for k, v := range details {
			if ok, i := toInt64(v); ok {
				metrics["prompt_"+k] = i
			}
		}
	}

	return metrics
}
