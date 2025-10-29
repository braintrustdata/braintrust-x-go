package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"

	braintrust "github.com/braintrustdata/braintrust-x-go"
	bttrace "github.com/braintrustdata/braintrust-x-go/trace"
)

func main() {
	// Create TracerProvider
	tp := trace.NewTracerProvider()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Set as global tracer provider so otel.Tracer() works
	otel.SetTracerProvider(tp)

	// Create Braintrust client with the TracerProvider
	_, err := braintrust.New(tp,
		braintrust.WithProject("rewrite-test"),
		braintrust.WithBlockingLogin(true),
	)
	if err != nil {
		log.Fatalf("Failed to create Braintrust client: %v", err)
	}

	// Demonstrate manual tracing with two spans
	demonstrateManualTracing()

	log.Println("\nExample complete - check Braintrust UI for traces")
}

func demonstrateManualTracing() {
	tracer := otel.Tracer("rewrite-example")
	ctx := context.Background()

	// Span 1: Parent operation
	_, span := tracer.Start(ctx, "parent_operation")
	defer span.End()
	span.SetAttributes(
		attribute.String("example.type", "parent"),
		attribute.Int("example.id", 1),
	)

	// Generate permalink
	if link, err := bttrace.Permalink(span); err == nil {
		log.Printf("\nView trace: %s", link)
	}
}
