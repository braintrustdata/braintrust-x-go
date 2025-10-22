// Package main demonstrates manual attachment usage in Braintrust traces.
//
// This example shows how to manually create and log attachments using the
// attachment package. Most users won't need to do this manually, as the
// instrumentation middleware (traceopenai, traceanthropic) automatically
// handles attachment conversion.
//
// To run this example:
//
//	export BRAINTRUST_API_KEY="your-api-key"
//	go run main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/attachment"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	// Create a tracer provider and enable Braintrust tracing on it
	// This gives us direct access to the provider for flushing
	tp := sdktrace.NewTracerProvider()
	err := trace.Enable(tp, braintrust.WithBlockingLogin(true))
	if err != nil {
		log.Fatalf("Failed to enable tracing: %v", err)
	}
	otel.SetTracerProvider(tp)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	tracer := otel.Tracer("attachments-example")
	ctx := context.Background()

	// Create a parent span to wrap all examples
	ctx, span := tracer.Start(ctx, "main.go")

	// Example 1: Create attachment from local file
	fmt.Println("Example 1: FromFile")
	exampleFromFile(ctx, tracer)

	// Example 2: Create attachment from io.Reader
	fmt.Println("\nExample 2: FromReader")
	exampleFromReader(ctx, tracer)

	// Example 3: Create attachment from bytes
	fmt.Println("\nExample 3: FromBytes")
	exampleFromBytes(ctx, tracer)

	// Example 4: Create attachment from base64 data
	fmt.Println("\nExample 4: FromBase64")
	exampleFromBase64(ctx, tracer)

	// Example 5: Create attachment from URL
	fmt.Println("\nExample 5: FromURL")
	exampleFromURL(ctx, tracer)

	// Example 6: Use attachment in manual span logging
	fmt.Println("\nExample 6: Manual span with attachment")
	exampleManualSpan(ctx, tracer)

	fmt.Println("\n✓ All examples completed successfully!")

	// End the main span
	span.End()

	// Force flush all spans to ensure they're sent to Braintrust
	if err := tp.ForceFlush(context.Background()); err != nil {
		log.Printf("Failed to flush tracer provider: %v", err)
	}

	// Get a link to the span in Braintrust
	link, err := trace.Permalink(span)
	if err != nil {
		fmt.Printf("Error getting permalink: %v\n", err)
	} else {
		fmt.Printf("\n🔗 View in Braintrust: %s\n", link)
	}
}

func exampleFromFile(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "attachment.from_file")
	defer span.End()

	// Create attachment from file
	att, err := attachment.FromFile(attachment.ImagePNG, "./test-image.png")
	if err != nil {
		log.Printf("Failed to create attachment from file: %v", err)
		return
	}

	fmt.Printf("  Created attachment from file\n")
	fmt.Printf("  Type: %s\n", att.Type)
	fmt.Printf("  Content length: %d bytes\n", len(att.Content))

	// Log as JSON attribute
	logAttachment(span, att)
}

func exampleFromReader(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "attachment.from_reader")
	defer span.End()

	// Open file as a reader
	file, err := os.Open("./test-image.png")
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	// Create attachment from reader
	att, err := attachment.FromReader(attachment.ImagePNG, file)
	if err != nil {
		log.Printf("Failed to create attachment from reader: %v", err)
		return
	}

	fmt.Printf("  Created attachment from reader\n")
	fmt.Printf("  Type: %s\n", att.Type)

	logAttachment(span, att)
}

func exampleFromBytes(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "attachment.from_bytes")
	defer span.End()

	// Read file into bytes
	imageBytes, err := os.ReadFile("./test-image.png")
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		return
	}

	// Create attachment from bytes
	att := attachment.FromBytes(attachment.ImagePNG, imageBytes)

	fmt.Printf("  Created attachment from %d bytes\n", len(imageBytes))
	fmt.Printf("  Type: %s\n", att.Type)

	logAttachment(span, att)
}

func exampleFromBase64(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "attachment.from_base64")
	defer span.End()

	// Use already base64-encoded data (tiny 1x1 transparent PNG)
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	// Create attachment from base64
	att := attachment.FromBase64(attachment.ImagePNG, base64Data)

	fmt.Printf("  Created attachment from base64 data\n")
	fmt.Printf("  Type: %s\n", att.Type)

	logAttachment(span, att)
}

func exampleFromURL(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "attachment.from_url")
	defer span.End()

	// Fetch image from URL
	// Using Braintrust's GitHub avatar as a public test image
	// The content type is automatically derived from the HTTP response headers
	url := "https://avatars.githubusercontent.com/u/109710255?s=200&v=4"

	att, err := attachment.FromURL(url)
	if err != nil {
		log.Printf("Failed to fetch URL: %v", err)
		return
	}

	fmt.Printf("  Fetched attachment from URL\n")
	fmt.Printf("  Type: %s\n", att.Type)

	logAttachment(span, att)
}

func exampleManualSpan(ctx context.Context, tracer oteltrace.Tracer) {
	_, span := tracer.Start(ctx, "vision.analyze_image")
	defer span.End()

	// Create attachment
	att, err := attachment.FromFile(attachment.ImagePNG, "./test-image.png")
	if err != nil {
		log.Printf("Failed to create attachment: %v", err)
		return
	}

	// Construct a message with text and image (similar to OpenAI/Anthropic format)
	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "What's in this image? Describe it in detail.",
				},
				att, // Attachment automatically marshals to correct format
			},
		},
	}

	// Log the messages as input
	messagesJSON, _ := json.Marshal(messages)
	span.SetAttributes(attribute.String("braintrust.input_json", string(messagesJSON)))

	// Simulate output
	output := []map[string]interface{}{
		{
			"role":    "assistant",
			"content": "This is a test image showing a simple geometric shape.",
		},
	}

	outputJSON, _ := json.Marshal(output)
	span.SetAttributes(attribute.String("braintrust.output_json", string(outputJSON)))

	// Add metadata
	metadata := map[string]interface{}{
		"model":    "vision-model",
		"provider": "custom",
	}
	metadataJSON, _ := json.Marshal(metadata)
	span.SetAttributes(attribute.String("braintrust.metadata", string(metadataJSON)))

	fmt.Printf("  Created manual span with attachment\n")
	fmt.Printf("  Span name: vision.analyze_image\n")
	fmt.Printf("  Input: text + image attachment\n")
	fmt.Printf("  Output: assistant response\n")
}

// Helper function to log attachment to span
func logAttachment(span oteltrace.Span, att *attachment.Attachment) {
	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Example with attachment",
				},
				att,
			},
		},
	}

	messagesJSON, _ := json.Marshal(messages)
	jsonStr := string(messagesJSON)

	// Debug: Print first 200 chars to verify attachment is in JSON
	if len(jsonStr) > 200 {
		fmt.Printf("  JSON (first 200 chars): %s...\n", jsonStr[:200])
	} else {
		fmt.Printf("  JSON: %s\n", jsonStr)
	}

	span.SetAttributes(attribute.String("braintrust.input_json", jsonStr))
}
