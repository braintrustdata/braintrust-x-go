// Golden test suite for LangChainGo integration
// This matches the TypeScript golden tests to ensure feature parity
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracelangchaingo"
)

func runGoldenTests() {
	fmt.Println("=== Braintrust LangChainGo Golden Test Suite ===")

	// Initialize Braintrust tracing with blocking login
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create handler
	handler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "gpt-4o-mini",
		Provider: "openai",
	})

	// Get tracer
	tracer := otel.Tracer("langchaingo-golden")

	// Create root span
	ctx, rootSpan := tracer.Start(context.Background(), "golden-tests")
	defer rootSpan.End()

	// Run tests
	testSystemPrompt(ctx, tracer, handler)
	testTemperatureVariations(ctx, tracer, handler)
	testVeryShortMaxTokens(ctx, tracer, handler)
	testPrefill(ctx, tracer, handler)
	testToolUseWithResult(ctx, tracer, handler)

	fmt.Println("\n=== All golden tests completed! ===")

	// Get permalink
	link, err := trace.Permalink(rootSpan)
	if err != nil {
		fmt.Printf("Error getting permalink: %v\n", err)
	} else {
		fmt.Printf("\nView all traces: %s\n", link)
	}
}

// testSystemPrompt validates system prompt handling
func testSystemPrompt(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "test-system-prompt")
	defer span.End()

	fmt.Println("\nTest: System Prompt")
	fmt.Println("-------------------")

	llm, err := openai.New(openai.WithCallback(handler))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Create messages with system prompt
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a pirate. Always respond in pirate speak."),
		llms.TextParts(llms.ChatMessageTypeHuman, "Tell me about the weather."),
	}

	resp, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(resp.Choices) > 0 {
		content := resp.Choices[0].Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("Response: %s\n", content)
	}
}

// testTemperatureVariations tests different temperature/top_p settings
func testTemperatureVariations(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "test-temperature-variations")
	defer span.End()

	fmt.Println("\nTest: Temperature Variations")
	fmt.Println("---------------------------")

	configs := []struct {
		temperature float64
		topP        float64
	}{
		{0.0, 1.0},
		{1.0, 0.9},
		{0.7, 0.95},
	}

	for _, config := range configs {
		fmt.Printf("Config: temp=%.1f, top_p=%.2f\n", config.temperature, config.topP)

		llm, err := openai.New(
			openai.WithCallback(handler),
			openai.WithModel("gpt-4o-mini"),
		)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		messages := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, "Say something creative about coding."),
		}

		resp, err := llm.GenerateContent(ctx, messages,
			llms.WithTemperature(config.temperature),
			llms.WithTopP(config.topP),
		)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Content
			if len(content) > 50 {
				content = content[:50] + "..."
			}
			fmt.Printf("Response: %s\n\n", content)
		}
	}
}

// testVeryShortMaxTokens tests very short token limits
func testVeryShortMaxTokens(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "test-very-short-max-tokens")
	defer span.End()

	fmt.Println("\nTest: Very Short Max Tokens")
	fmt.Println("--------------------------")

	llm, err := openai.New(
		openai.WithCallback(handler),
		openai.WithModel("gpt-4o-mini"),
	)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is AI?"),
	}

	resp, err := llm.GenerateContent(ctx, messages, llms.WithMaxTokens(5))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("Response (5 tokens max): %s\n", resp.Choices[0].Content)
		fmt.Printf("Stop reason: %s\n", resp.Choices[0].StopReason)
	}
}

// testPrefill tests prefilling with AI message
func testPrefill(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "test-prefill")
	defer span.End()

	fmt.Println("\nTest: Prefill")
	fmt.Println("------------")

	llm, err := openai.New(openai.WithCallback(handler))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Start with partial AI response to guide completion
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Write a haiku about coding."),
		llms.TextParts(llms.ChatMessageTypeAI, "Here is a haiku:"),
	}

	resp, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("Prefilled response: Here is a haiku: %s\n", resp.Choices[0].Content)
	}
}

// testToolUseWithResult tests multi-turn tool usage with result processing
func testToolUseWithResult(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "test-tool-use-with-result")
	defer span.End()

	fmt.Println("\nTest: Tool Use With Result (Multi-turn)")
	fmt.Println("--------------------------------------")

	llm, err := openai.New(openai.WithCallback(handler))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Create calculator tool
	calculator := tools.Calculator{
		CallbacksHandler: handler,
	}

	// First turn: Ask question that requires calculation
	fmt.Println("Turn 1: Ask a math question")
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is 123 multiplied by 456?"),
	}

	resp1, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(resp1.Choices) > 0 {
		fmt.Printf("AI response: %s\n", resp1.Choices[0].Content)

		// If AI suggests using calculator, do the calculation
		result, err := calculator.Call(ctx, "123 * 456")
		if err != nil {
			log.Printf("Calculator error: %v", err)
			return
		}

		fmt.Printf("Calculator result: %s\n", result)

		// Second turn: Provide result back to AI
		fmt.Println("\nTurn 2: Provide calculation result")
		messages = append(messages,
			llms.TextParts(llms.ChatMessageTypeAI, resp1.Choices[0].Content),
			llms.TextParts(llms.ChatMessageTypeHuman, fmt.Sprintf("The calculator says the result is %s. Can you confirm this is correct?", result)),
		)

		resp2, err := llm.GenerateContent(ctx, messages)
		if err != nil {
			log.Printf("Error: %v", err)
			return
		}

		if len(resp2.Choices) > 0 {
			content := resp2.Choices[0].Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("AI confirmation: %s\n", content)
		}
	}
}
