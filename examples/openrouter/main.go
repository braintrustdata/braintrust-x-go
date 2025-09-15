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

	fmt.Println("Using api key", os.Getenv("OPENROUTER_API_KEY")[0:6]+"...")

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant."),
			openai.UserMessage("What is the capital of France?"),
		},
		Model: "openai/gpt-3.5-turbo", // OpenRouter model format
	}

	resp, err := client.Chat.Completions.New(context.Background(), params)
	if err != nil {
		log.Fatal(err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		log.Fatal("no response content")
	}

	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)
	fmt.Println("\n✅ OpenRouter example completed successfully!")
	fmt.Println("Check your Braintrust dashboard to view the trace.")
}
