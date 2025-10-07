// This example demonstrates how to manually log LLM data to Braintrust
// without using middleware. This is useful, for example, if you have
// your own internal AI proxy and want to instrument that.
//
// All fields are documented here:
//  https://www.braintrust.dev/docs/integrations/opentelemetry#manual-tracing

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	bttrace "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	fmt.Println("🧠 Manual LLM Logging Example")

	// Initialize Braintrust tracing
	teardown, err := bttrace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	ctx := context.Background()

	// Example 1: Simple LLM call
	simpleExample(ctx)

	// Example 2: Multi-turn conversation
	conversationExample(ctx)

	// Example 3: LLM call with tool/function calling
	toolCallingExample(ctx)

	fmt.Println("\n✅ All examples logged! Check your Braintrust dashboard to view the traces.")
}

// simpleExample shows a basic LLM call with all the key attributes
func simpleExample(ctx context.Context) {
	fmt.Println("\n📝 Example 1: Simple LLM Call")

	tracer := otel.Tracer("manual-llm-example")
	ctx, span := tracer.Start(ctx, "llm.chat.completions")
	defer span.End()

	// 1. Set input messages
	messages := []map[string]any{
		{"role": "system", "content": "You are a helpful assistant."},
		{"role": "user", "content": "What is the capital of France?"},
	}
	setJSONAttr(span, "braintrust.input_json", messages)

	// 2. Set metadata (model parameters)
	metadata := map[string]any{
		"model":       "gpt-4o-mini",
		"temperature": 0.7,
		"max_tokens":  100,
		"provider":    "openai",
	}
	setJSONAttr(span, "braintrust.metadata", metadata)

	// 3. Set span attributes to mark this as an LLM span
	spanAttrs := map[string]string{
		"type": "llm",
	}
	setJSONAttr(span, "braintrust.span_attributes", spanAttrs)

	// Simulate calling an LLM (replace this with your actual LLM call)
	output := []map[string]any{
		{
			"role":    "assistant",
			"content": "The capital of France is Paris.",
		},
	}

	// 4. Set output (typically an array of messages with assistant response)
	setJSONAttr(span, "braintrust.output_json", output)

	// 5. Set metrics (token usage)
	metrics := map[string]any{
		"prompt_tokens":     15,
		"completion_tokens": 8,
		"total_tokens":      23,
	}
	setJSONAttr(span, "braintrust.metrics", metrics)

	fmt.Println("✓ Logged simple LLM call")
}

// conversationExample shows logging a multi-turn conversation
func conversationExample(ctx context.Context) {
	fmt.Println("\n💬 Example 2: Multi-turn Conversation")

	tracer := otel.Tracer("manual-llm-example")
	ctx, span := tracer.Start(ctx, "llm.chat.completions")
	defer span.End()

	// Input includes conversation history
	messages := []map[string]any{
		{"role": "system", "content": "You are a helpful math tutor."},
		{"role": "user", "content": "What is 5 + 3?"},
		{"role": "assistant", "content": "5 + 3 equals 8."},
		{"role": "user", "content": "And what is that times 2?"},
	}
	setJSONAttr(span, "braintrust.input_json", messages)

	metadata := map[string]any{
		"model":    "gpt-4o-mini",
		"provider": "openai",
	}
	setJSONAttr(span, "braintrust.metadata", metadata)

	spanAttrs := map[string]string{
		"type": "llm",
	}
	setJSONAttr(span, "braintrust.span_attributes", spanAttrs)

	output := []map[string]any{
		{
			"role":    "assistant",
			"content": "8 times 2 equals 16.",
		},
	}
	setJSONAttr(span, "braintrust.output_json", output)

	metrics := map[string]any{
		"prompt_tokens":     42,
		"completion_tokens": 9,
		"total_tokens":      51,
	}
	setJSONAttr(span, "braintrust.metrics", metrics)

	fmt.Println("✓ Logged multi-turn conversation")
}

// toolCallingExample shows logging an LLM call with function/tool calling
func toolCallingExample(ctx context.Context) {
	fmt.Println("\n🔧 Example 3: LLM with Tool Calling")

	tracer := otel.Tracer("manual-llm-example")
	ctx, span := tracer.Start(ctx, "llm.chat.completions")
	defer span.End()

	messages := []map[string]any{
		{"role": "user", "content": "What's the weather in San Francisco?"},
	}
	setJSONAttr(span, "braintrust.input_json", messages)

	// Include tool definitions in metadata
	metadata := map[string]any{
		"model":    "gpt-4o-mini",
		"provider": "openai",
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	}
	setJSONAttr(span, "braintrust.metadata", metadata)

	spanAttrs := map[string]string{
		"type": "llm",
	}
	setJSONAttr(span, "braintrust.span_attributes", spanAttrs)

	// Output with tool call
	output := []map[string]any{
		{
			"role": "assistant",
			"tool_calls": []map[string]any{
				{
					"id":   "call_123",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": `{"location": "San Francisco"}`,
					},
				},
			},
		},
	}
	setJSONAttr(span, "braintrust.output_json", output)

	metrics := map[string]any{
		"prompt_tokens":     85,
		"completion_tokens": 20,
		"total_tokens":      105,
	}
	setJSONAttr(span, "braintrust.metrics", metrics)

	fmt.Println("✓ Logged LLM call with tool calling")
}

// setJSONAttr marshals a value to JSON and sets it as a span attribute
func setJSONAttr(span trace.Span, key string, value any) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		log.Printf("Warning: failed to marshal %s: %v", key, err)
		return
	}
	span.SetAttributes(attribute.String(key, string(jsonBytes)))
}
