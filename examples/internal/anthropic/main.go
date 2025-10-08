// Anthropic kitchen sink - tests all the Anthropic features with minimal code
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceanthropic"
)

var tracer = otel.Tracer("anthropic-examples")

// AnthropicBot demonstrates using Anthropic messages API with tracing
type AnthropicBot struct {
	client anthropic.Client
}

func newAnthropicBot(client anthropic.Client) *AnthropicBot {
	return &AnthropicBot{
		client: client,
	}
}

// messages demonstrates basic non-streaming message
func (a *AnthropicBot) messages(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "messages")
	defer span.End()

	fmt.Println("\n=== Example 1: Messages ===")

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: "You are a helpful assistant."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is the capital of France?")),
		},
		Temperature: anthropic.Float(0.7),
	})
	if err != nil {
		return fmt.Errorf("messages error: %v", err)
	}

	fmt.Printf("  %s\n", msg.Content[0].Text)
	return nil
}

// tools demonstrates tools with non-streaming
func (a *AnthropicBot) tools(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "tools")
	defer span.End()

	fmt.Println("\n=== Example 2: Tools ===")

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: "You are a helpful weather assistant."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in SF?")),
		},
		Temperature:   anthropic.Float(0.7),
		TopP:          anthropic.Float(0.9),
		TopK:          anthropic.Int(50),
		StopSequences: []string{"END"},
		Tools: []anthropic.ToolUnionParam{
			anthropic.ToolUnionParamOfTool(
				anthropic.ToolInputSchemaParam{
					Properties: map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state",
						},
					},
					Required: []string{"location"},
				},
				"get_weather",
			),
		},
	})
	if err != nil {
		return fmt.Errorf("tools error: %v", err)
	}

	for _, content := range msg.Content {
		switch content.Type {
		case "text":
			fmt.Printf("  Text: %s\n", content.Text)
		case "tool_use":
			fmt.Printf("  Tool: %s\n", content.Name)
			fmt.Printf("  Input: %v\n", content.Input)
		}
	}

	return nil
}

// streaming demonstrates streaming with tools
func (a *AnthropicBot) streaming(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "streaming")
	defer span.End()

	fmt.Println("\n=== Example 3: Streaming ===")

	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model: anthropic.ModelClaude3_7SonnetLatest,
		System: []anthropic.TextBlockParam{
			{Text: "You are a helpful assistant."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in Tokyo and tell me a joke.")),
		},
		MaxTokens:   1024,
		Temperature: anthropic.Float(0.8),
		TopP:        anthropic.Float(0.95),
		Tools: []anthropic.ToolUnionParam{
			anthropic.ToolUnionParamOfTool(
				anthropic.ToolInputSchemaParam{
					Properties: map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and country",
						},
					},
					Required: []string{"location"},
				},
				"get_weather",
			),
		},
	})

	fmt.Print("  ")
	for stream.Next() {
		event := stream.Current()
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockStartEvent:
			if eventVariant.ContentBlock.Type == "tool_use" {
				fmt.Printf("\n  [Tool: %s] ", eventVariant.ContentBlock.Name)
			}
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				fmt.Print(deltaVariant.Text)
			}
		}
	}
	fmt.Println()

	if err := stream.Err(); err != nil {
		return fmt.Errorf("streaming error: %v", err)
	}

	return nil
}

// extendedThinking demonstrates Claude's extended thinking capability
func (a *AnthropicBot) extendedThinking(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "extended-thinking")
	defer span.End()

	fmt.Println("\n=== Example 4: Extended Thinking ===")

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 16000,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is the capital of France and why is it historically significant?")),
		},
		Thinking: anthropic.ThinkingConfigParamOfEnabled(10000),
	})
	if err != nil {
		return fmt.Errorf("extended thinking error: %v", err)
	}

	for _, content := range msg.Content {
		switch content.Type {
		case "thinking":
			thinking := content.Thinking
			if len(thinking) > 100 {
				thinking = thinking[:100] + "..."
			}
			fmt.Printf("  Thinking: %s\n", thinking)
		case "text":
			fmt.Printf("  Response: %s\n", content.Text)
		}
	}

	return nil
}

func main() {
	fmt.Println("Braintrust Anthropic Tracing Examples")
	fmt.Println("======================================")

	// Initialize braintrust tracing with a specific project
	teardown, err := trace.Quickstart(braintrust.WithDefaultProject("go-sdk-internal-examples"))
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create an Anthropic client with tracing middleware
	client := anthropic.NewClient(
		option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")), // defaults to os.LookupEnv("ANTHROPIC_API_KEY")
		option.WithMiddleware(traceanthropic.Middleware),
	)

	ctx := context.Background()

	// Set the experiment as parent for tracing
	ctx, rootSpan := tracer.Start(ctx, "anthropic-examples")
	defer rootSpan.End()

	// ======================
	// ANTHROPIC MESSAGES EXAMPLES
	// ======================
	fmt.Println("\nAnthropic Messages Examples")
	fmt.Println("===========================")
	fmt.Println("Demonstrating: system prompts, tools, parameters, streaming & non-streaming")

	bot := newAnthropicBot(client)

	if err := bot.messages(ctx); err != nil {
		log.Printf("Error: %v", err)
	}

	if err := bot.tools(ctx); err != nil {
		log.Printf("Error: %v", err)
	}

	if err := bot.streaming(ctx); err != nil {
		log.Printf("Error: %v", err)
	}

	if err := bot.extendedThinking(ctx); err != nil {
		log.Printf("Error: %v", err)
	}

	fmt.Println("\n=== Tracing Complete ===")
	fmt.Println("All examples completed successfully!")
	fmt.Println("Check your Braintrust dashboard to view the traces.")
}
