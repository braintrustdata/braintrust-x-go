package oteltest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
	"github.com/braintrust/braintrust-x-go/braintrust/internal"
)

// Setup sets up otel tracing for testing (no sampling, sync, stores spans in memory)
// and returns a Tracer and an Exporter that can be used to flush the spans.
func Setup(t *testing.T, opts ...sdktrace.TracerProviderOption) (oteltrace.Tracer, *Exporter) {
	t.Helper()
	internal.FailTestsOnWarnings(t)

	// setup otel to be fully synchronous
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	opts = append(opts,
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(processor), // flushes immediately
	)

	tp := sdktrace.NewTracerProvider(opts...)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.GetTracerProvider().Tracer(t.Name())

	t.Cleanup(func() {
		diag.ClearLogger()
		// withoutcancel is a workaround for usetesting linter which is otherwise
		// kinda useful https://github.com/ldez/usetesting/issues/4
		ctx := context.WithoutCancel(t.Context())

		err := tp.Shutdown(ctx)
		if err != nil {
			t.Errorf("Error shutting down tracer provider: %v", err)
		}
		otel.SetTracerProvider(original)
	})

	return tracer, &Exporter{exporter: exporter, t: t}
}

// Exporter is a wrapper around the OTel InMemoryExporter that provides some
// helper functions for testing.
type Exporter struct {
	exporter *tracetest.InMemoryExporter
	t        *testing.T
}

// Flush returns the spans buffered in memory.
func (e *Exporter) Flush() []Span {
	stubs := e.exporter.GetSpans()
	e.exporter.Reset()

	spans := make([]Span, len(stubs))
	for i, span := range stubs {
		spans[i] = Span{t: e.t, Stub: span}
	}

	return spans
}

// FlushOne returns the first span buffered in memory and fails if there is more
// than one span
func (e *Exporter) FlushOne() Span {
	e.t.Helper()
	spans := e.Flush()
	if len(spans) != 1 {
		e.t.Fatalf("Expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

// Span is a wrapper around the OTel SpanStub with some helpful
// testing functions.
type Span struct {
	t    *testing.T
	Stub tracetest.SpanStub
}

// Name returns the span's name.
func (s *Span) Name() string {
	return s.Stub.Name
}

// AssertAttrEquals asserts that the attribute is equal to the expected value.
func (s *Span) AssertAttrEquals(key string, expected any) {
	s.t.Helper()
	attr := s.Attr(key)
	attr.AssertEquals(expected)
}

// Attrs returns all the span's attributes matching the key.
func (s *Span) Attrs(key string) []Attr {
	attrs := []Attr{}
	for _, attr := range s.Stub.Attributes {
		if string(attr.Key) == key {
			attrs = append(attrs, Attr{t: s.t, Key: string(attr.Key), Value: attr.Value})
		}
	}
	return attrs
}

// Attr return the attribute matching the key and fails if there isn't
// exactly one.
func (s *Span) Attr(key string) Attr {
	s.t.Helper()
	attrs := s.Attrs(key)
	require.Len(s.t, attrs, 1)
	return attrs[0]
}

// Map returns a map containing critical span attributes for use in testing.
func (s *Span) Map() map[string]interface{} {
	return map[string]interface{}{
		"name":       s.Stub.Name,
		"spanKind":   s.Stub.SpanKind.String(),
		"attributes": convertAttributes(s.Stub.Attributes),
		"events":     convertEvents(s.Stub.Events),
		"status": map[string]interface{}{
			"code":        s.Stub.Status.Code.String(),
			"description": s.Stub.Status.Description,
		},
	}
}

// Snapshot returns a JSON string containing critical span attributes
// for use in testing assertions.
func (s *Span) Snapshot() string {
	jsonBytes, err := json.MarshalIndent(s.Map(), "", "  ")
	if err != nil {
		s.t.Fatalf("Failed to marshal span snapshot: %v", err)
	}
	return string(jsonBytes)
}

func convertAttributes(attrs []attr.KeyValue) map[string]interface{} {
	result := make(map[string]interface{})
	for _, a := range attrs {
		result[string(a.Key)] = convertAttributeValue(a.Value)
	}
	return result
}

func convertAttributeValue(value attr.Value) interface{} {
	switch value.Type() {
	case attr.BOOL:
		return value.AsBool()
	case attr.INT64:
		return value.AsInt64()
	case attr.FLOAT64:
		return value.AsFloat64()
	case attr.STRING:
		return value.AsString()
	case attr.BOOLSLICE:
		return value.AsBoolSlice()
	case attr.INT64SLICE:
		return value.AsInt64Slice()
	case attr.FLOAT64SLICE:
		return value.AsFloat64Slice()
	case attr.STRINGSLICE:
		return value.AsStringSlice()
	default:
		return value.AsString() // fallback
	}
}

func convertEvents(events []sdktrace.Event) []map[string]interface{} {
	result := make([]map[string]interface{}, len(events))
	for i, event := range events {
		result[i] = map[string]interface{}{
			"name":       event.Name,
			"attributes": convertAttributes(event.Attributes),
		}
	}
	return result
}

// Attr is a wrapper around the OTel Attribute with some helpful
// testing functions.
type Attr struct {
	t     *testing.T
	Key   string
	Value attr.Value
}

// String returns the attribute as a string and fails if the attribute is not a string.
func (a Attr) String() string {
	require.Equal(a.t, a.Value.Type(), attr.STRING)
	return a.Value.AsString()
}

// AssertEquals asserts that the attribute is equal to the expected value.
func (a Attr) AssertEquals(expected any) {
	a.t.Helper()
	switch v := expected.(type) {
	case string:
		assert.Equal(a.t, v, a.String())
	case int64:
		assert.Equal(a.t, v, a.Value.AsInt64())
	case float64:
		assert.Equal(a.t, v, a.Value.AsFloat64())
	case bool:
		assert.Equal(a.t, v, a.Value.AsBool())
	default:
		assert.Failf(a.t, "unsupported type", "expected type %T is not supported", expected)
	}
}
