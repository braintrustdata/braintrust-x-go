// This example demonstrates basic Google Gemini tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"google.golang.org/genai"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracegenai"
)

func main() {
	fmt.Println("Braintrust Google Gemini Basic Example")

	// Initialize Braintrust tracing with blocking login to ensure permalinks work immediately
	teardown, err := trace.Quickstart(
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Create Gemini client with Braintrust tracing
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		HTTPClient: tracegenai.Client(), // Add tracing via custom HTTP client
		APIKey:     os.Getenv("GOOGLE_API_KEY"),
		Backend:    genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Get a tracer instance
	tracer := otel.Tracer("genai-example")

	// Create a parent span to wrap the Gemini call
	ctx, span := tracer.Start(context.Background(), "ask-question")
	defer span.End()

	// Make a simple generateContent request
	resp, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.0-flash-exp",
		genai.Text("What is the capital of France?"),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.Text())

	// Get a link to the span in Braintrust
	link, _ := trace.Permalink(span)
	fmt.Printf("View trace: %s\n", link)
}
