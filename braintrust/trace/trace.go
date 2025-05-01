package trace

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

// Quickstart will configure the OpenTelemetry tracer to
// an easy way of getting up and running if you are new to OpenTelemetry. It
// returns a teardown function that should be called before your program exits.
func Quickstart() (teardown func(), err error) {

	diag.Debugf("Initializing OpenTelemetry tracer with experiment_id: %s", os.Getenv("BRAINTRUST_EXPERIMENT_ID"))

	// Create Braintrust OTLP exporter
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint("api.braintrust.dev"),
			otlptracehttp.WithURLPath("/otel/v1/traces"),
			otlptracehttp.WithHeaders(map[string]string{
				"Authorization": "Bearer " + os.Getenv("BRAINTRUST_API_KEY"),
				"x-bt-parent":   "experiment_id:" + os.Getenv("BRAINTRUST_EXPERIMENT_ID"),
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create a tracer provider with both exporters
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)

	teardown = func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			diag.Warnf("Error shutting down tracer provider: %v", err)
		}
	}

	return teardown, nil
}

// ParentContextKey is the used to store information about a span's parent object (e.g.
// project, experiment) in a context.Context object.
type ParentContextKey string
