// This example demonstrates basic OpenAI tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
)

func main() {
	fmt.Println("🧠 Braintrust OpenAI Basic Example")

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create OpenAI client with Braintrust tracing middleware
	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	ctx := context.Background()

	// Make a simple chat completion request
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is the capital of France?"),
		},
		Model: openai.ChatModelGPT4oMini,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.Choices[0].Message.Content)
	fmt.Println("\n✅ Request completed! Check your Braintrust dashboard to view the trace.")
}
