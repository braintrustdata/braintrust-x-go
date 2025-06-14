package oteltest

import (
	"context"
	"reflect"
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

// SetupTracer sets up otel for testing (no sampling, sync, stores spans in memory)
// and returns an Exporter that can be used to flush the spans.
func SetupTracer(t *testing.T, opts ...sdktrace.TracerProviderOption) (oteltrace.Tracer, *Exporter) {
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

	t.Cleanup(func() {
		diag.ClearLogger()
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Errorf("Error shutting down tracer provider: %v", err)
		}
		otel.SetTracerProvider(original)
	})

	tracer := otel.GetTracerProvider().Tracer(t.Name())

	return tracer, &Exporter{
		exporter: exporter,
		t:        t,
	}
}

// Exporter is a wrapper around the OTel InMemoryExporter that provides some
// helper functions for testing.
type Exporter struct {
	exporter *tracetest.InMemoryExporter
	t        *testing.T
}

// Flush returns the spans buffered in memory.
func (e *Exporter) Flush() []Span {
	spans := e.exporter.GetSpans()
	e.exporter.Reset()

	spanObjs := make([]Span, len(spans))
	for i, span := range spans {
		spanObjs[i] = Span{t: e.t, Stub: span}
	}

	return spanObjs
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
	switch reflect.TypeOf(expected) {
	case reflect.TypeOf("string"):
		assert.Equal(a.t, a.String(), expected)
	case reflect.TypeOf(int64(1)):
		assert.Equal(a.t, a.Value.AsInt64(), expected)
	case reflect.TypeOf(float64(1.0)):
		assert.Equal(a.t, a.Value.AsFloat64(), expected)
	case reflect.TypeOf(true):
		assert.Equal(a.t, a.Value.AsBool(), expected)
	default:
		assert.Fail(a.t, "unsupported type: %s", reflect.TypeOf(expected))
	}
}
