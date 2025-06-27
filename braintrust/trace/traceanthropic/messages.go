package traceanthropic

// this file parses the messages API.

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/trace/internal"
)

// messagesTracer is a tracer for the anthropic v1/messages POST endpoint.
// See docs here: https://docs.anthropic.com/en/api/messages
type messagesTracer struct {
	streaming bool
	metadata  map[string]any
}

func newMessagesTracer() *messagesTracer {
	return &messagesTracer{
		streaming: false,
		metadata: map[string]any{
			"provider": "anthropic",
			"endpoint": "/v1/messages",
		},
	}
}

func (mt *messagesTracer) StartSpan(ctx context.Context, t time.Time, request io.Reader) (context.Context, trace.Span, error) {
	ctx, span := tracer().Start(
		ctx,
		"anthropic.messages.create",
		trace.WithTimestamp(t),
	)

	var raw map[string]interface{}
	if err := json.NewDecoder(request).Decode(&raw); err != nil {
		return ctx, span, err
	}

	metadataFields := []string{
		"model",
		"max_tokens",
		"temperature",
		"top_p",
		"top_k",
		"stop_sequences",
		"stream",
		"system",
		"tools",
		"tool_choice",
		"metadata",
		"container",
		"mcp_servers",
		"service_tier",
		"thinking",
	}

	// handle simple fields here.
	for _, field := range metadataFields {
		if value, exists := raw[field]; exists {
			mt.metadata[field] = value
			// keep track of streaming requests so we can parse the streaming response later.
			if field == "stream" {
				if value, ok := value.(bool); ok {
					mt.streaming = value
				}
			}
		}
	}

	if messages, ok := raw["messages"]; ok {
		if err := internal.SetJSONAttr(span, "braintrust.input", messages); err != nil {
			return ctx, span, err
		}
	}

	if err := internal.SetJSONAttr(span, "braintrust.metadata", mt.metadata); err != nil {
		return ctx, span, err
	}

	return ctx, span, nil
}

func (mt *messagesTracer) TagSpan(span trace.Span, body io.Reader) error {
	if mt.streaming {
		return mt.parseStreamingResponse(span, body)
	}
	return mt.parseResponse(span, body)
}

func (mt *messagesTracer) parseStreamingResponse(span trace.Span, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	var allResults []map[string]any
	usage := make(map[string]any)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		line = strings.TrimPrefix(line, "data: ")
		if line == "[DONE]" {
			break // End of stream
		}

		var chunk map[string]any
		err := json.Unmarshal([]byte(line), &chunk)
		if err != nil {
			return err
		}

		allResults = append(allResults, chunk)

		// Handle usage in streaming response from different event types
		eventType, ok := chunk["type"].(string)
		if ok {
			switch eventType {

			// Contains input and cache tokens
			case "message_start":
				// Usage is nested in message object for message_start events
				if message, ok := chunk["message"].(map[string]any); ok {
					if curUsage, ok := message["usage"].(map[string]any); ok {
						// Initialize combined usage with message_start data (contains input_tokens)
						for k, v := range curUsage {
							usage[k] = v
						}
					}
				}

			// Contains output tokens, There can be multiple "message_delta" events in a single response.
			// But the usage data in there is supposed to be cumulative as per the docs.
			// So using the last usage data is fine.
			case "message_delta":
				// Usage is at top level for message_delta events (contains final output_tokens)
				if curUsage, ok := chunk["usage"].(map[string]any); ok {
					// message_delta usage is cumulative, so it overrides any previous values
					for k, v := range curUsage {
						usage[k] = v
					}
				}
			}
		}
	}

	// Post-process streaming results to match expected output format
	output := mt.postprocessStreamingResults(allResults)
	if output != nil {
		if err := internal.SetJSONAttr(span, "braintrust.output", output); err != nil {
			return err
		}
	}

	// Handle usage metrics
	if len(usage) > 0 {
		metrics := parseUsageTokens(usage)
		if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (mt *messagesTracer) postprocessStreamingResults(allResults []map[string]any) []map[string]interface{} {
	var content strings.Builder
	var stopReason interface{}

	for _, result := range allResults {
		eventType, ok := result["type"].(string)
		if !ok {
			continue
		}

		switch eventType {
		case "content_block_delta":
			if delta, ok := result["delta"].(map[string]any); ok {
				if text, ok := delta["text"].(string); ok {
					content.WriteString(text)
				}
			}
		case "message_delta":
			if delta, ok := result["delta"].(map[string]any); ok {
				if sr, ok := delta["stop_reason"]; ok {
					stopReason = sr
				}
			}
		}
	}

	// Store stop reason in metadata if present
	if stopReason != nil {
		mt.metadata["stop_reason"] = stopReason
	}

	// Build the final response content block
	contentBlocks := []map[string]interface{}{
		{
			"type": "text",
			"text": content.String(),
		},
	}

	return contentBlocks
}

func (mt *messagesTracer) parseResponse(span trace.Span, body io.Reader) error {
	var raw map[string]interface{}
	err := json.NewDecoder(body).Decode(&raw)
	if err != nil {
		return err
	}

	return mt.handleMessageResponse(span, raw)
}

func (mt *messagesTracer) handleMessageResponse(span trace.Span, rawMsg map[string]any) error {
	metadataFields := []string{
		"id",
		"type",
		"role",
		"model",
		"stop_reason",
		"stop_sequence",
	}

	for _, field := range metadataFields {
		if v, ok := rawMsg[field]; ok {
			mt.metadata[field] = v
		}
	}

	if err := internal.SetJSONAttr(span, "braintrust.metadata", mt.metadata); err != nil {
		return err
	}

	if usage, ok := rawMsg["usage"].(map[string]any); ok {
		metrics := parseUsageTokens(usage)
		if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			return err
		}
	}

	if content, ok := rawMsg["content"]; ok {
		if err := internal.SetJSONAttr(span, "braintrust.output", content); err != nil {
			return err
		}
	}

	return nil
}
