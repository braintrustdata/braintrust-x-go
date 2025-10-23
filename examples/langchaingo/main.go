// Example demonstrating basic LangChainGo tracing with Braintrust
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracelangchaingo"
)

func main() {
	fmt.Println("=== Braintrust LangChainGo Simple Example ===\n")

	// Step 1: Initialize Braintrust tracing with blocking login
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Step 2: Create the Braintrust callback handler
	// Optionally provide model and provider information for richer traces
	handler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "gpt-4o-mini",
		Provider: "openai",
	})

	// Step 3: Create LangChainGo LLM with the callback handler
	llm, err := openai.New(openai.WithCallback(handler))
	if err != nil {
		log.Fatal(err)
	}

	// Step 4: Create a root span to group all operations
	tracer := otel.Tracer("langchaingo-example")
	ctx, rootSpan := tracer.Start(context.Background(), "examples/langchaingo/main")

	// Simple completion
	fmt.Println("Simple LLM call:")
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is the capital of France?"),
	}
	resp, err := llm.GenerateContent(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}
	if len(resp.Choices) > 0 {
		fmt.Printf("Answer: %s\n\n", resp.Choices[0].Content)
	}

	// Multi-turn conversation
	fmt.Println("Multi-turn conversation:")
	conversation := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Hi, my name is Alice."),
	}
	resp, err = llm.GenerateContent(ctx, conversation)
	if err != nil {
		log.Fatal(err)
	}
	if len(resp.Choices) > 0 {
		firstResponse := resp.Choices[0].Content
		fmt.Printf("AI: %s\n", firstResponse)

		// Continue conversation
		conversation = append(conversation,
			llms.TextParts(llms.ChatMessageTypeAI, firstResponse),
			llms.TextParts(llms.ChatMessageTypeHuman, "What did I tell you my name was?"),
		)
		resp, err = llm.GenerateContent(ctx, conversation)
		if err != nil {
			log.Fatal(err)
		}
		if len(resp.Choices) > 0 {
			fmt.Printf("AI: %s\n\n", resp.Choices[0].Content)
		}
	}

	// End the root span
	rootSpan.End()

	fmt.Println("✓ All calls traced to Braintrust!")

	// Print the permalink to view traces
	link, err := trace.Permalink(rootSpan)
	if err != nil {
		fmt.Printf("Error getting permalink: %v\n", err)
	} else {
		fmt.Printf("\nView traces: %s\n", link)
	}
}
