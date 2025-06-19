package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"go.opentelemetry.io/otel"

	"github.com/braintrust/braintrust-x-go/braintrust"
	"github.com/braintrust/braintrust-x-go/braintrust/api"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceanthropic"
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

func (a *AnthropicBot) simpleMessageWithTemperature(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "simpleMessageWithTemperature")
	defer span.End()

	fmt.Println("\n=== Example 1: Simple Message with Temperature ===")

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model: anthropic.ModelClaude3_7SonnetLatest,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is the capital of France?")),
		},
		MaxTokens:   1024,
		Temperature: anthropic.Float(0.7), // This will show up in metadata
	})
	if err != nil {
		return fmt.Errorf("simple message error: %v", err)
	}

	fmt.Printf("Response: %s\n\n", message.Content[0].Text)
	return nil
}

func (a *AnthropicBot) streamingResponseWithParameters(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "streamingResponseWithParameters")
	defer span.End()

	fmt.Println("=== Example 2: Streaming Response with Parameters ===")

	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model: anthropic.ModelClaude3_7SonnetLatest,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Tell me a short joke.")),
		},
		MaxTokens:   1024,
		Temperature: anthropic.Float(0.8),  // Higher temperature for more creative output
		TopP:        anthropic.Float(0.95), // This will also show up in metadata
	})

	fmt.Print("Streaming response: ")
	for stream.Next() {
		event := stream.Current()
		switch eventVariant := event.AsAny().(type) {
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

func (a *AnthropicBot) systemPromptWithAdvancedParameters(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "systemPromptWithAdvancedParameters")
	defer span.End()

	fmt.Println("\n=== Example 3: System Prompt with Advanced Parameters ===")

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: "You are a helpful assistant that responds in a friendly and concise manner."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello! How are you doing today?")),
		},
		Temperature:   anthropic.Float(0.5), // Lower temperature for more focused responses
		TopP:          anthropic.Float(0.9),
		TopK:          anthropic.Int(50),
		StopSequences: []string{"END", "STOP"}, // This will show up in metadata
	})
	if err != nil {
		return fmt.Errorf("system prompt error: %v", err)
	}

	fmt.Printf("Response: %s\n\n", message.Content[0].Text)
	return nil
}

func (a *AnthropicBot) multiTurnConversation(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "multiTurnConversation")
	defer span.End()

	fmt.Println("=== Example 4: Multi-turn Conversation ===")

	// First turn
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What is my first name?")),
	}

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		Messages:  messages,
		MaxTokens: 1024,
	})
	if err != nil {
		return fmt.Errorf("multi-turn conversation first turn error: %v", err)
	}

	fmt.Printf("Claude: %s\n", message.Content[0].Text)

	// Continue the conversation
	messages = append(messages, message.ToParam())
	messages = append(messages, anthropic.NewUserMessage(
		anthropic.NewTextBlock("My name is Alice. What did I just tell you?"),
	))

	message, err = a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		Messages:  messages,
		MaxTokens: 1024,
	})
	if err != nil {
		return fmt.Errorf("multi-turn conversation second turn error: %v", err)
	}

	fmt.Printf("Claude: %s\n", message.Content[0].Text)
	return nil
}

func main() {
	fmt.Println("🧠 Braintrust Anthropic Tracing Examples")
	fmt.Println("=========================================")

	// Initialize braintrust tracing with a specific project
	projectName := "traceanthropic-example-test"
	project, err := api.RegisterProject(projectName)
	if err != nil {
		log.Fatal(err)
	}

	opt := braintrust.WithDefaultProjectID(project.ID)

	teardown, err := trace.Quickstart(opt)
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

	// Register experiment
	experiment, err := api.RegisterExperiment("anthropic-examples", project.ID)
	if err != nil {
		log.Fatal(err)
	}

	// Set the experiment as parent for tracing
	ctx = trace.SetParent(ctx, trace.NewExperiment(experiment.ID))
	fmt.Printf("Using project: %s (%s), experiment: %s\n", project.Name, project.ID, experiment.ID)

	ctx, rootSpan := tracer.Start(ctx, "anthropic-examples")
	defer rootSpan.End()

	// ======================
	// ANTHROPIC MESSAGES EXAMPLES
	// ======================
	fmt.Println("\n💬 Anthropic Messages Examples")
	fmt.Println("==============================")

	bot := newAnthropicBot(client)

	// Example 1: Simple message completion with temperature
	if err := bot.simpleMessageWithTemperature(ctx); err != nil {
		log.Printf("Error in simple message example: %v", err)
	}

	// Example 2: Streaming response with multiple parameters
	if err := bot.streamingResponseWithParameters(ctx); err != nil {
		log.Printf("Error in streaming example: %v", err)
	}

	// Example 3: Conversation with system prompt and parameters
	if err := bot.systemPromptWithAdvancedParameters(ctx); err != nil {
		log.Printf("Error in system prompt example: %v", err)
	}

	// Example 4: Multi-turn conversation
	if err := bot.multiTurnConversation(ctx); err != nil {
		log.Printf("Error in multi-turn conversation example: %v", err)
	}

	fmt.Println("\n=== Tracing Complete ===")
	fmt.Println("✅ All examples completed successfully!")
	fmt.Println("Check your Braintrust dashboard to view the traces.")
}
