// This example demonstrates adding Braintrust tracing to an existing OpenTelemetry setup.
// It shows how to use trace.Enable() to add Braintrust to an app that already has
// other exporters (console exporter in this case).
package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	log.Println("🔧 Setting up existing OpenTelemetry infrastructure...")

	// Create a tracer providor with an existing processor / exporter
	tp := sdktrace.NewTracerProvider()
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter))
	otel.SetTracerProvider(tp)

	// Enable Braintrust tracing
	err := trace.Enable(tp)
	if err != nil {
		log.Fatalf("❌ Failed to enable Braintrust tracing: %v", err)
	}

	tracer := otel.Tracer("otel-enable-demo")
	_, span := tracer.Start(context.Background(), "demo-operation")
	span.End()
}
