// This example demonstrates adding Braintrust tracing to an existing OpenTelemetry setup.
// It shows how to use trace.Enable() to add Braintrust to an app that already has
// other exporters (console exporter in this case).

package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	log.Println("🔧 Setting up existing OpenTelemetry infrastructure...")

	// Create a tracer providor with an existing processor / exporter
	tp := sdktrace.NewTracerProvider()
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	otel.SetTracerProvider(tp)

	// Enable Braintrust tracing with blocking login to ensure permalinks work
	err := trace.Enable(tp, braintrust.WithBlockingLogin(true))
	if err != nil {
		log.Fatalf("❌ Failed to enable Braintrust tracing: %v", err)
	}

	log.Println("✅ Braintrust tracing enabled successfully")

	tracer := otel.Tracer("otel-enable-demo")
	_, span := tracer.Start(context.Background(), "demo-operation")
	span.End()

	// Print permalink so user can view the trace in the UI
	link, err := trace.Permalink(span)
	if err != nil {
		log.Printf("⚠️  Could not generate permalink: %v", err)
	} else {
		fmt.Printf("\n🔗 View trace: %s\n", link)
	}
}
