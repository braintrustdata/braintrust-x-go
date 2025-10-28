// This example uses a forked version of langchaingo that adds Anthropic callback support.
// The fork is at github.com/clutchski/langchaingo (anthropic-callbacks branch).
// This is not the official version of langchaingo and is used for testing purposes only.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracelangchaingo"
)

var tracer = otel.Tracer("langchaingo-anthropic-example")

func main() {
	fmt.Println("Braintrust LangChainGo Anthropic & OpenAI Example")
	fmt.Println("==================================================")

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart(
		braintrust.WithDefaultProject("langchaingo-anthropic-example"),
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	ctx := context.Background()

	// Create root span for the entire example
	ctx, rootSpan := tracer.Start(ctx, "langchaingo-anthropic-openai")
	defer rootSpan.End()

	// Create Anthropic LLM with Braintrust tracing
	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	anthropicHandler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "claude-3-5-sonnet-20241022",
		Provider: "anthropic",
	})

	anthropicLLM, err := anthropic.New(
		anthropic.WithToken(anthropicAPIKey),
		anthropic.WithModel("claude-3-5-sonnet-20241022"),
		anthropic.WithCallback(anthropicHandler),
	)
	if err != nil {
		log.Fatalf("Failed to create Anthropic LLM: %v", err)
	}

	// Create OpenAI LLM with Braintrust tracing
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	openaiHandler := tracelangchaingo.NewHandlerWithOptions(tracelangchaingo.HandlerOptions{
		Model:    "gpt-4o",
		Provider: "openai",
	})

	openaiLLM, err := openai.New(
		openai.WithToken(openaiAPIKey),
		openai.WithModel("gpt-4o"),
		openai.WithCallback(openaiHandler),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenAI LLM: %v", err)
	}

	// Test queries
	queries := []string{
		"What is the capital of France?",
		"Explain quantum computing in one sentence.",
		"What is 2+2?",
	}

	fmt.Println("Running traced queries against Anthropic and OpenAI...")
	fmt.Println()

	for i, query := range queries {
		fmt.Printf("Query %d: %s\n", i+1, query)
		fmt.Println("---")

		// Query Anthropic
		fmt.Println("Anthropic Response:")
		anthropicResp, err := llms.GenerateFromSinglePrompt(ctx, anthropicLLM, query)
		if err != nil {
			log.Printf("Anthropic error: %v", err)
		} else {
			fmt.Printf("%s\n\n", anthropicResp)
		}

		// Query OpenAI
		fmt.Println("OpenAI Response:")
		openaiResp, err := llms.GenerateFromSinglePrompt(ctx, openaiLLM, query)
		if err != nil {
			log.Printf("OpenAI error: %v", err)
		} else {
			fmt.Printf("%s\n\n", openaiResp)
		}

		fmt.Println()
	}

	fmt.Println("\n=== Tracing Complete ===")
	fmt.Println("All queries completed successfully!")

	// Print permalink to the root span
	link, err := trace.Permalink(rootSpan)
	if err != nil {
		fmt.Printf("Error generating permalink: %v\n", err)
	} else {
		fmt.Printf("View trace: %s\n", link)
	}
}
