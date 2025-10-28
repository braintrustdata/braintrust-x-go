// This example demonstrates side-by-side comparison of OpenAI and Anthropic LLMs
// using LangChainGo with Braintrust tracing.
// It runs the same prompts through both providers to compare their responses.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracelangchaingo"
)

func main() {
	fmt.Println("=== LangChainGo OpenAI vs Anthropic Comparison ===")
	fmt.Println()

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create handlers for both providers
	openaiHandler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "gpt-4o-mini",
		Provider: "openai",
	})

	anthropicHandler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "claude-3-5-sonnet-20241022",
		Provider: "anthropic",
	})

	// Create LLM instances
	openaiLLM, err := openai.New(openai.WithCallback(openaiHandler))
	if err != nil {
		log.Fatalf("Error creating OpenAI LLM: %v", err)
	}

	anthropicLLM, err := anthropic.New(anthropic.WithCallback(anthropicHandler))
	if err != nil {
		log.Fatalf("Error creating Anthropic LLM: %v", err)
	}

	// Get tracer
	tracer := otel.Tracer("langchaingo-compare")

	// Create root span
	ctx, rootSpan := tracer.Start(context.Background(), "examples/langchaingo-compare")
	defer rootSpan.End()

	// Print permalink early so user can check traces
	if link, err := trace.Permalink(rootSpan); err != nil {
		fmt.Printf("Error getting permalink: %v\n", err)
	} else {
		fmt.Printf("View traces: %s\n\n", link)
	}

	// Run comparison examples
	fmt.Println("1. Simple Question")
	fmt.Println(strings.Repeat("=", 80))
	simpleQuestion(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n2. Creative Writing")
	fmt.Println(strings.Repeat("=", 80))
	creativeWriting(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n3. Multi-turn Conversation")
	fmt.Println(strings.Repeat("=", 80))
	multiTurnConversation(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n4. System Prompt (Personality)")
	fmt.Println(strings.Repeat("=", 80))
	systemPrompt(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n5. Temperature Variation")
	fmt.Println(strings.Repeat("=", 80))
	temperatureVariation(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n6. Short Token Limit")
	fmt.Println(strings.Repeat("=", 80))
	shortTokenLimit(ctx, tracer, openaiLLM, anthropicLLM)

	fmt.Println("\n7. Streaming Response")
	fmt.Println(strings.Repeat("=", 80))
	streamingResponse(ctx, tracer, openaiLLM, anthropicLLM)

	// Get permalink
	link, err := trace.Permalink(rootSpan)
	if err != nil {
		fmt.Printf("\nError getting permalink: %v\n", err)
	} else {
		fmt.Printf("\nView all traces: %s\n", link)
	}
}

func simpleQuestion(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "simple-question")
	defer span.End()

	question := "What is the capital of France?"
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, question),
	}

	fmt.Printf("Q: %s\n\n", question)

	// OpenAI
	fmt.Println("OpenAI (gpt-4o-mini):")
	respOpenAI, err := openaiLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respOpenAI.Choices) > 0 {
		fmt.Printf("  %s\n", respOpenAI.Choices[0].Content)
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic (claude-3-5-sonnet):")
	respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respAnthropic.Choices) > 0 {
		fmt.Printf("  %s\n", respAnthropic.Choices[0].Content)
	}
}

func creativeWriting(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "creative-writing")
	defer span.End()

	prompt := "Write a haiku about programming."
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	fmt.Printf("Prompt: %s\n\n", prompt)

	// OpenAI
	fmt.Println("OpenAI:")
	respOpenAI, err := openaiLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respOpenAI.Choices) > 0 {
		fmt.Printf("%s\n", indent(respOpenAI.Choices[0].Content, 2))
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic:")
	respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respAnthropic.Choices) > 0 {
		fmt.Printf("%s\n", indent(respAnthropic.Choices[0].Content, 2))
	}
}

func multiTurnConversation(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "multi-turn")
	defer span.End()

	// First turn
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "I'm learning Go. What's a good first project?"),
	}

	fmt.Println("Turn 1: I'm learning Go. What's a good first project?")
	fmt.Println()

	// OpenAI
	fmt.Println("OpenAI:")
	respOpenAI1, err := openaiLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}
	var openaiResp1 string
	if len(respOpenAI1.Choices) > 0 {
		openaiResp1 = respOpenAI1.Choices[0].Content
		fmt.Printf("%s\n", indent(truncate(openaiResp1, 150), 2))
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic:")
	respAnthropic1, err := anthropicLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}
	var anthropicResp1 string
	if len(respAnthropic1.Choices) > 0 {
		anthropicResp1 = respAnthropic1.Choices[0].Content
		fmt.Printf("%s\n", indent(truncate(anthropicResp1, 150), 2))
	}

	// Second turn
	fmt.Println("\nTurn 2: How long will that take to build?")
	fmt.Println()

	messagesOpenAI := append(messages,
		llms.TextParts(llms.ChatMessageTypeAI, openaiResp1),
		llms.TextParts(llms.ChatMessageTypeHuman, "How long will that take to build?"),
	)

	messagesAnthropic := append([]llms.MessageContent{},
		llms.TextParts(llms.ChatMessageTypeHuman, "I'm learning Go. What's a good first project?"),
		llms.TextParts(llms.ChatMessageTypeAI, anthropicResp1),
		llms.TextParts(llms.ChatMessageTypeHuman, "How long will that take to build?"),
	)

	// OpenAI
	fmt.Println("OpenAI:")
	respOpenAI2, err := openaiLLM.GenerateContent(ctx, messagesOpenAI)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respOpenAI2.Choices) > 0 {
		fmt.Printf("%s\n", indent(truncate(respOpenAI2.Choices[0].Content, 150), 2))
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic:")
	respAnthropic2, err := anthropicLLM.GenerateContent(ctx, messagesAnthropic)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respAnthropic2.Choices) > 0 {
		fmt.Printf("%s\n", indent(truncate(respAnthropic2.Choices[0].Content, 150), 2))
	}
}

func systemPrompt(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "system-prompt")
	defer span.End()

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a helpful pirate. Always respond in pirate speak."),
		llms.TextParts(llms.ChatMessageTypeHuman, "Tell me about the weather today."),
	}

	fmt.Println("System: You are a helpful pirate. Always respond in pirate speak.")
	fmt.Println("User: Tell me about the weather today.")
	fmt.Println()

	// OpenAI
	fmt.Println("OpenAI:")
	respOpenAI, err := openaiLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respOpenAI.Choices) > 0 {
		fmt.Printf("%s\n", indent(truncate(respOpenAI.Choices[0].Content, 200), 2))
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic:")
	respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respAnthropic.Choices) > 0 {
		fmt.Printf("%s\n", indent(truncate(respAnthropic.Choices[0].Content, 200), 2))
	}
}

func temperatureVariation(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "temperature-variation")
	defer span.End()

	prompt := "Complete this sentence: The future of AI is"
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	temperatures := []float64{0.0, 0.7, 1.5}

	fmt.Printf("Prompt: %s\n\n", prompt)

	for _, temp := range temperatures {
		fmt.Printf("Temperature = %.1f:\n", temp)

		// OpenAI
		fmt.Println("  OpenAI:")
		respOpenAI, err := openaiLLM.GenerateContent(ctx, messages, llms.WithTemperature(temp))
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
		} else if len(respOpenAI.Choices) > 0 {
			fmt.Printf("    %s\n", truncate(respOpenAI.Choices[0].Content, 100))
		}

		// Anthropic
		fmt.Println("  Anthropic:")
		respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages, llms.WithTemperature(temp))
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
		} else if len(respAnthropic.Choices) > 0 {
			fmt.Printf("    %s\n", truncate(respAnthropic.Choices[0].Content, 100))
		}

		fmt.Println()
	}
}

func shortTokenLimit(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "short-token-limit")
	defer span.End()

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Explain quantum computing in detail."),
	}

	fmt.Println("Prompt: Explain quantum computing in detail.")
	fmt.Println("Max tokens: 10")
	fmt.Println()

	// OpenAI
	fmt.Println("OpenAI:")
	respOpenAI, err := openaiLLM.GenerateContent(ctx, messages, llms.WithMaxTokens(10))
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respOpenAI.Choices) > 0 {
		fmt.Printf("  %s\n", respOpenAI.Choices[0].Content)
		fmt.Printf("  Stop reason: %s\n", respOpenAI.Choices[0].StopReason)
	}

	fmt.Println()

	// Anthropic
	fmt.Println("Anthropic:")
	respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages, llms.WithMaxTokens(10))
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if len(respAnthropic.Choices) > 0 {
		fmt.Printf("  %s\n", respAnthropic.Choices[0].Content)
		fmt.Printf("  Stop reason: %s\n", respAnthropic.Choices[0].StopReason)
	}
}

func streamingResponse(parentCtx context.Context, tracer oteltrace.Tracer, openaiLLM *openai.LLM, anthropicLLM *anthropic.LLM) {
	ctx, span := tracer.Start(parentCtx, "streaming")
	defer span.End()

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Count from 1 to 5."),
	}

	fmt.Println("Prompt: Count from 1 to 5.")
	fmt.Println()

	// OpenAI
	fmt.Print("OpenAI (streaming): ")
	streamingFunc := func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}
	respOpenAI, err := openaiLLM.GenerateContent(ctx, messages, llms.WithStreamingFunc(streamingFunc))
	if err != nil {
		fmt.Printf("\n  Error: %v\n", err)
	} else if len(respOpenAI.Choices) > 0 {
		fmt.Printf("\n  (Accumulated %d chars)\n", len(respOpenAI.Choices[0].Content))
	}

	fmt.Println()

	// Anthropic
	fmt.Print("Anthropic (streaming): ")
	respAnthropic, err := anthropicLLM.GenerateContent(ctx, messages, llms.WithStreamingFunc(streamingFunc))
	if err != nil {
		fmt.Printf("\n  Error: %v\n", err)
	} else if len(respAnthropic.Choices) > 0 {
		fmt.Printf("\n  (Accumulated %d chars)\n", len(respAnthropic.Choices[0].Content))
	}
}

// Helper functions
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
