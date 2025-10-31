package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	temporal "github.com/braintrustdata/braintrust-x-go/examples/temporal"
)

func main() {
	// Initialize Braintrust tracing
	tp := sdktrace.NewTracerProvider()
	err := trace.Enable(tp)
	if err != nil {
		log.Fatalln("Unable to initialize Braintrust tracing:", err)
	}
	defer tp.Shutdown(context.Background())

	// Set the tracer provider globally
	otel.SetTracerProvider(tp)

	// Configure propagators for distributed tracing
	// Required to propagate braintrust.parent across process boundaries
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer for Temporal
	tracer := otel.Tracer("temporal-braintrust")

	// Create OpenTelemetry interceptor for Temporal
	tracingInterceptor, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{
		Tracer: tracer,
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal tracing interceptor:", err)
	}

	// Create Temporal client with OpenTelemetry interceptor
	c, err := client.Dial(client.Options{
		HostPort:     client.DefaultHostPort,
		Interceptors: []interceptor.ClientInterceptor{tracingInterceptor},
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client:", err)
	}
	defer c.Close()

	// Create worker
	w := worker.New(c, temporal.TaskQueue, worker.Options{})

	// Register workflow and activities
	w.RegisterWorkflow(temporal.ProcessWorkflow)
	activities := &temporal.Activities{}
	w.RegisterActivity(activities.Process)

	// Start listening to the Task Queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker:", err)
	}
}
