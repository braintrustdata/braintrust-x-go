// This example demonstrates comprehensive LangChainGo tracing with Braintrust.
// It shows how to trace:
// - Simple LLM calls
// - Multi-turn conversations
// - Chain executions
// - Tool usage
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracelangchaingo"
)

func main() {
	fmt.Println("=== Braintrust LangChainGo Comprehensive Example ===\n")

	// Initialize Braintrust tracing with blocking login to ensure permalinks work immediately
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create the Braintrust callback handler
	handler := tracelangchaingo.NewHandler()

	// Create LangChainGo OpenAI LLM with callback
	llm, err := openai.New(openai.WithCallback(handler))
	if err != nil {
		log.Fatal(err)
	}

	// Get a tracer instance
	tracer := otel.Tracer("langchaingo-example")

	// Create a root span that will contain all examples
	ctx, rootSpan := tracer.Start(context.Background(), "langchaingo-examples")

	// Example 1: Simple LLM call
	fmt.Println("1. Simple LLM Call")
	fmt.Println("------------------")
	simpleLLMCall(ctx, tracer, llm)

	// Example 2: Multi-turn conversation
	fmt.Println("\n2. Multi-turn Conversation")
	fmt.Println("--------------------------")
	multiTurnConversation(ctx, tracer, llm)

	// Example 3: Simulated chain execution
	fmt.Println("\n3. Simulated Chain Execution")
	fmt.Println("----------------------------")
	simulatedChain(ctx, tracer, handler, llm)

	// Example 4: Simulated tool usage
	fmt.Println("\n4. Simulated Tool Usage")
	fmt.Println("-----------------------")
	simulatedTool(ctx, tracer, handler)

	// End the root span
	rootSpan.End()

	fmt.Println("\n=== All examples completed! ===")

	// Get a link to the root span in Braintrust
	link, err := trace.Permalink(rootSpan)
	if err != nil {
		fmt.Printf("Error getting permalink: %v\n", err)
	} else {
		fmt.Printf("\nView all traces: %s\n", link)
	}
}

// simpleLLMCall demonstrates basic LLM tracing
func simpleLLMCall(parentCtx context.Context, tracer oteltrace.Tracer, llm *openai.LLM) {
	ctx, span := tracer.Start(parentCtx, "simple-question")
	defer span.End()

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is the capital of France?"),
	}

	resp, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(resp.Choices) > 0 {
		fmt.Printf("Q: What is the capital of France?\n")
		fmt.Printf("A: %s\n", resp.Choices[0].Content)
	}
}

// multiTurnConversation demonstrates tracing a conversation with context
func multiTurnConversation(parentCtx context.Context, tracer oteltrace.Tracer, llm *openai.LLM) {
	ctx, span := tracer.Start(parentCtx, "multi-turn-conversation")
	defer span.End()

	// First turn
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "I'm planning a trip to Paris. What should I see?"),
	}

	resp1, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Turn 1:\n")
	fmt.Printf("User: I'm planning a trip to Paris. What should I see?\n")
	if len(resp1.Choices) > 0 {
		response1 := resp1.Choices[0].Content
		// Truncate for display
		if len(response1) > 100 {
			response1 = response1[:100] + "..."
		}
		fmt.Printf("Assistant: %s\n\n", response1)

		// Second turn - add context
		messages = append(messages,
			llms.TextParts(llms.ChatMessageTypeAI, resp1.Choices[0].Content),
			llms.TextParts(llms.ChatMessageTypeHuman, "How many days should I spend there?"),
		)
	}

	resp2, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Turn 2:\n")
	fmt.Printf("User: How many days should I spend there?\n")
	if len(resp2.Choices) > 0 {
		response2 := resp2.Choices[0].Content
		if len(response2) > 100 {
			response2 = response2[:100] + "..."
		}
		fmt.Printf("Assistant: %s\n", response2)
	}
}

// simulatedChain demonstrates chain callback tracing
func simulatedChain(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler, llm *openai.LLM) {
	ctx, span := tracer.Start(parentCtx, "summarize-chain")
	defer span.End()

	// Simulate chain start
	chainInputs := map[string]any{
		"text": "LangChain is a framework for developing applications powered by language models.",
		"task": "summarize",
	}
	handler.HandleChainStart(ctx, chainInputs)

	// Make LLM call within the chain
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman,
			"Summarize this in 5 words: LangChain is a framework for developing applications powered by language models."),
	}

	resp, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		handler.HandleChainError(ctx, err)
		log.Printf("Error: %v", err)
		return
	}

	summary := ""
	if len(resp.Choices) > 0 {
		summary = resp.Choices[0].Content
		fmt.Printf("Input: %s\n", chainInputs["text"])
		fmt.Printf("Summary: %s\n", summary)
	}

	// Simulate chain end
	chainOutputs := map[string]any{
		"summary": summary,
	}
	handler.HandleChainEnd(ctx, chainOutputs)
}

// simulatedTool demonstrates tool callback tracing
func simulatedTool(parentCtx context.Context, tracer oteltrace.Tracer, handler *tracelangchaingo.Handler) {
	ctx, span := tracer.Start(parentCtx, "weather-tool")
	defer span.End()

	// Simulate tool start
	toolInput := "Boston, MA"
	handler.HandleToolStart(ctx, toolInput)

	// Simulate tool execution (mock weather lookup)
	fmt.Printf("Tool: get_weather\n")
	fmt.Printf("Input: %s\n", toolInput)

	// Mock tool response
	toolOutput := `{"temperature": 72, "condition": "sunny", "location": "Boston, MA"}`
	fmt.Printf("Output: %s\n", toolOutput)

	// Simulate tool end
	handler.HandleToolEnd(ctx, toolOutput)
}
