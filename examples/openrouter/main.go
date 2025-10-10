// This example demonstrates tracing OpenRouter calls with the openai client.
// Set the env var OPENROUTER_API_KEY to your OpenRouter API key and run the example.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

const projectName = "openrouter-example"

func main() {
	teardown, err := trace.Quickstart(
		braintrust.WithDefaultProject(projectName),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Login is only required to view links.
	if _, err = braintrust.Login(); err != nil {
		log.Fatal(err)
	}

	// The key insight: OpenAI client works with OpenRouter, but you need to:
	// 1. Use the OpenRouter API key directly (it handles Bearer prefix automatically)
	// 2. Set the correct base URL
	// 3. Use OpenRouter's model naming convention (e.g., "openai/gpt-3.5-turbo")
	// 4. Include the optional OpenRouter headers

	// Note: traceopenai.Middleware has compatibility issues with OpenRouter
	// Using manual tracing instead
	client := openai.NewClient(
		option.WithBaseURL("https://openrouter.ai/api/v1"),
		option.WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		option.WithHeader("HTTP-Referer", "https://github.com/braintrustdata/braintrust-x-go"),
		option.WithHeader("X-Title", "Braintrust Go SDK Example"),
	)

	// Get a tracer instance
	tracer := otel.Tracer("openrouter-example")

	// Create a parent span to wrap the OpenRouter call
	ctx, span := tracer.Start(context.Background(), "ask-question")
	defer span.End()

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant."),
			openai.UserMessage("What is the capital of France?"),
		},
		Model: "openai/gpt-3.5-turbo", // OpenRouter model format
	}

	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		log.Fatal("no response content")
	}

	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)

	// Get a link to the span in Braintrust
	link, _ := trace.Permalink(span)
	fmt.Printf("View trace: %s\n", link)
}
