package main

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	braintrust "github.com/braintrustdata/braintrust-x-go"
	"github.com/braintrustdata/braintrust-x-go/trace"
)

func main() {
	// Create Braintrust client with OpenTelemetry auto-setup
	bt, err := braintrust.NewWithOtel(
		braintrust.WithAPIKey(os.Getenv("BRAINTRUST_API_KEY")),
		braintrust.WithProject("rewrite-test"),
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatalf("Failed to create Braintrust client: %v", err)
	}
	defer func() {
		if err := bt.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown Braintrust client: %v", err)
		}
	}()

	log.Println(bt)

	// Demonstrate manual tracing with two spans
	demonstrateManualTracing()

	log.Println("\nExample complete - check Braintrust UI for traces")
}

func demonstrateManualTracing() {
	tracer := otel.Tracer("rewrite-example")
	ctx := context.Background()

	// Set project parent context
	ctx = trace.SetParent(ctx, trace.Parent{
		Type: trace.ParentTypeProjectName,
		ID:   "rewrite-test",
	})

	// Span 1: Parent operation
	ctx, parentSpan := tracer.Start(ctx, "parent_operation")
	parentSpan.SetAttributes(
		attribute.String("example.type", "parent"),
		attribute.Int("example.id", 1),
	)

	// Span 2: Child operation
	_, childSpan := tracer.Start(ctx, "child_operation")
	childSpan.SetAttributes(
		attribute.String("example.type", "child"),
		attribute.String("status", "complete"),
	)
	childSpan.End()

	parentSpan.End()

	// Generate permalink
	if link, err := trace.Permalink(parentSpan); err == nil {
		log.Printf("\nView trace: %s", link)
	}
}
