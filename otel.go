package main

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() (*trace.TracerProvider, error) {
	// Create stdout exporter for local debugging
	stdoutExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// Create Braintrust OTLP exporter
	braintrustExporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint("api.braintrust.dev"),
			otlptracehttp.WithURLPath("/otel/v1/traces"),
			otlptracehttp.WithHeaders(map[string]string{
				"Authorization": "Bearer " + os.Getenv("BRAINTRUST_API_KEY"),
				"x-bt-parent":   "project_id:" + os.Getenv("BRAINTRUST_PROJECT_ID"),
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create a tracer provider with both exporters
	tp := trace.NewTracerProvider(
		trace.WithBatcher(stdoutExporter),
		trace.WithBatcher(braintrustExporter),
	)
	otel.SetTracerProvider(tp)

	return tp, nil
}
