package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	bttrace "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	temporal "github.com/braintrustdata/braintrust-x-go/examples/temporal"
)

func main() {
	// Initialize Braintrust tracing
	tp := sdktrace.NewTracerProvider()
	err := bttrace.Enable(tp)
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

	// Task function that executes a workflow
	task := func(ctx context.Context, input int) (string, error) {
		workflowOptions := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("w-%d", input),
			TaskQueue: temporal.TaskQueue,
		}

		we, err := c.ExecuteWorkflow(ctx, workflowOptions, temporal.ProcessWorkflow, input)
		if err != nil {
			return "", fmt.Errorf("failed to execute workflow: %w", err)
		}

		var result string
		err = we.Get(ctx, &result)
		if err != nil {
			return "", fmt.Errorf("failed to get workflow result: %w", err)
		}

		return result, nil
	}

	// Run Braintrust eval
	_, err = eval.Run(context.Background(), eval.Opts[int, string]{
		Project:    temporal.ProjectName,
		Experiment: "temporal-distributed-tracing",
		Cases: eval.NewCases([]eval.Case[int, string]{
			{Input: 10, Expected: "processed number: 10"},
			{Input: 20, Expected: "processed number: 20"},
			{Input: 30, Expected: "processed number: 30"},
		}),
		Task: task,
		Scorers: []eval.Scorer[int, string]{
			eval.NewScorer("exact_match", func(_ context.Context, _ int, expected, result string, _ eval.Metadata) (eval.Scores, error) {
				score := 0.0
				if expected == result {
					score = 1.0
				}
				return eval.S(score), nil
			}),
		},
	})

	if err != nil {
		log.Fatalf("Eval failed: %v", err)
	}
}
