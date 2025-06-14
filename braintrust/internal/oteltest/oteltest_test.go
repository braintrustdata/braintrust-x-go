package oteltest

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestSpanSnapshot(t *testing.T) {
	tracer, exporter := Setup(t)

	_, span := tracer.Start(t.Context(), "test-span", oteltrace.WithSpanKind(oteltrace.SpanKindClient))

	// Add some attributes
	span.SetAttributes(
		attribute.String("string_attr", "test_value"),
		attribute.Int("int_attr", 42),
		attribute.Bool("bool_attr", true),
	)

	// Add an event
	span.AddEvent("test_event", oteltrace.WithAttributes(
		attribute.String("event_attr", "event_value"),
	))

	// Set status
	span.SetStatus(codes.Error, "test error")

	span.End()

	// Get the span and test snapshot
	testSpan := exporter.FlushOne()
	snapshot := testSpan.Snapshot()

	expected := `{
  "attributes": {
    "bool_attr": true,
    "int_attr": 42,
    "string_attr": "test_value"
  },
  "events": [
    {
      "attributes": {
        "event_attr": "event_value"
      },
      "name": "test_event"
    }
  ],
  "name": "test-span",
  "spanKind": "client",
  "status": {
    "code": "Error",
    "description": "test error"
  }
}`

	// Use a simple string comparison for now - in practice you'd use assert.JSONEq or similar
	if snapshot != expected {
		t.Errorf("Snapshot mismatch.\nGot:\n%s\n\nExpected:\n%s", snapshot, expected)
	}
}
