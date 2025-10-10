// This example demonstrates basic Anthropic tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceanthropic"
)

func main() {
	fmt.Println("Braintrust Anthropic Basic Example")

	// Initialize Braintrust tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Login is only required to view links.
	if _, err = braintrust.Login(); err != nil {
		log.Fatal(err)
	}

	// Create Anthropic client with Braintrust tracing middleware
	client := anthropic.NewClient(
		option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
		option.WithMiddleware(traceanthropic.Middleware),
	)

	// Get a tracer instance
	tracer := otel.Tracer("anthropic-example")

	// Create a parent span to wrap the Anthropic call
	ctx, span := tracer.Start(context.Background(), "ask-question")
	defer span.End()

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

	// Get a link to the span in Braintrust
	link, _ := trace.Permalink(span)
	fmt.Printf("View trace: %s\n", link)
}
