// This example demonstrates basic OpenAI tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"go.opentelemetry.io/otel"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
)

func main() {
	fmt.Println("Braintrust OpenAI Basic Example")

	// Initialize Braintrust tracing with blocking login to ensure permalinks work immediately
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create OpenAI client with Braintrust tracing middleware
	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	// Get a tracer instance
	tracer := otel.Tracer("openai-example")

	// Create a parent span to wrap the OpenAI call
	ctx, span := tracer.Start(context.Background(), "ask-question")
	defer span.End()

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

	// Get a link to the span in Braintrust
	link, err := trace.Permalink(span)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(link)
	}
}
