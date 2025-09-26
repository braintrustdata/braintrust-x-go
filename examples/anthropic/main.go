// This example demonstrates basic Anthropic tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceanthropic"
)

func main() {
	fmt.Println("🧠 Braintrust Anthropic Basic Example")

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create Anthropic client with Braintrust tracing middleware
	client := anthropic.NewClient(
		option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
		option.WithMiddleware(traceanthropic.Middleware),
	)

	ctx := context.Background()

	// Make a simple message request
	message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model: anthropic.ModelClaude3_7SonnetLatest,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is the capital of France?")),
		},
		MaxTokens: 1024,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", message.Content[0].Text)
	fmt.Println("\n✅ Request completed! Check your Braintrust dashboard to view the trace.")
}
