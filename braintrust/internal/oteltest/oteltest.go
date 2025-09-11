package oteltest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/internal"
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
		// Use background context for cleanup
		ctx := context.Background()

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

func NewExporter(t *testing.T) *Exporter {
	t.Helper()
	return &Exporter{exporter: tracetest.NewInMemoryExporter(), t: t}
}

// NewProcessor creates a simple span processor wrapping a memory exporter and returns both.
// This is useful for testing when you need a processor to pass to Enable().
func NewProcessor(t *testing.T) (sdktrace.SpanProcessor, *Exporter) {
	t.Helper()
	exporter := NewExporter(t)
	processor := sdktrace.NewSimpleSpanProcessor(exporter.InMemoryExporter())
	return processor, exporter
}

func (e *Exporter) InMemoryExporter() *tracetest.InMemoryExporter {
	return e.exporter
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

// testingT is a minimal interface for testing that can be implemented by testing.T and mocked for tests
type testingT interface {
	Helper()
	Errorf(format string, args ...interface{})
}

// FlushOne returns the first span buffered in memory and fails if there is not
// exactly one span.
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

// Status returns the span's status.
func (s *Span) Status() sdktrace.Status {
	return s.Stub.Status
}

// Events returns the span's events.
func (s *Span) Events() []sdktrace.Event {
	return s.Stub.Events
}

// AssertNameIs asserts that the span's name equals the expected name.
func (s *Span) AssertNameIs(expected string) {
	s.t.Helper()
	assert.Equal(s.t, expected, s.Stub.Name)
}

func (s *Span) AssertInTimeRange(tr TimeRange) {
	s.t.Helper()
	if tr.IsZero() {
		s.t.Errorf("TimeRange is zero - cannot assert span timing")
		return
	}
	stub := s.Stub
	assert.NotZero(s.t, tr.Start)
	assert.NotZero(s.t, tr.End)
	assert.True(s.t, tr.Start.Before(stub.StartTime))
	assert.True(s.t, tr.End.After(stub.EndTime))
	assert.True(s.t, stub.StartTime.Before(stub.EndTime))
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

// HasAttr returns true if the span has at least one attribute with the given key.
func (s *Span) HasAttr(key string) bool {
	return len(s.Attrs(key)) > 0
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
	a.t.Helper()
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

// Braintrust-specific span methods

// Input returns the Braintrust input from the span attributes.
// Supports both "braintrust.input" (OpenAI tracing) and "braintrust.input_json" (eval tracing).
func (s *Span) Input() any {
	s.t.Helper()
	var input any

	// Try both attribute name formats
	inputKeys := []string{"braintrust.input", "braintrust.input_json"}
	for _, key := range inputKeys {
		if s.HasAttr(key) {
			raw := s.Attr(key).String()
			err := json.Unmarshal([]byte(raw), &input)
			require.NoError(s.t, err)
			return input
		}
	}

	s.t.Fatalf("No input attribute found. Tried: %v", inputKeys)
	return nil
}

// Output returns the Braintrust output from the span attributes.
// Supports both "braintrust.output" (OpenAI tracing) and "braintrust.output_json" (eval tracing).
func (s *Span) Output() any {
	s.t.Helper()
	var output any

	// Try both attribute name formats
	outputKeys := []string{"braintrust.output", "braintrust.output_json"}
	for _, key := range outputKeys {
		if s.HasAttr(key) {
			raw := s.Attr(key).String()
			err := json.Unmarshal([]byte(raw), &output)
			require.NoError(s.t, err)
			return output
		}
	}

	s.t.Fatalf("No output attribute found. Tried: %v", outputKeys)
	return nil
}

// Metadata returns the Braintrust metadata from the span attributes.
func (s *Span) Metadata() map[string]any {
	s.t.Helper()
	var metadata map[string]any
	s.unmarshal("braintrust.metadata", &metadata)
	return metadata
}

// Metrics returns the Braintrust metrics from the span attributes.
func (s *Span) Metrics() map[string]float64 {
	s.t.Helper()
	var metrics map[string]float64
	s.unmarshal("braintrust.metrics", &metrics)
	return metrics
}

// unmarshal is a helper method to unmarshal JSON attributes.
func (s *Span) unmarshal(key string, into any) {
	s.t.Helper()
	attr := s.Attr(key)
	raw := attr.String()
	err := json.Unmarshal([]byte(raw), into)
	require.NoError(s.t, err)
}

type Timer struct {
	start time.Time
}

func NewTimer() *Timer {
	return &Timer{
		start: time.Now(),
	}
}

// Returns the time range since the timer was created to now.
func (t *Timer) Tick() TimeRange {
	return TimeRange{
		Start: t.start,
		End:   time.Now(),
	}
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

// IsZero returns true if the TimeRange is the zero value
func (tr TimeRange) IsZero() bool {
	return tr.Start.IsZero() && tr.End.IsZero()
}

// Event is a summary of an otel event.
type Event struct {
	Name  string
	Attrs map[string]any
}

// TestSpan is a span that is easier to write and compare to real otel spans
// in tests. Any missing attributes will be set to sane OTel defaults, like
// Unset status and empty events and attributes.
type TestSpan struct {
	Name              string
	Attrs             map[string]any
	JSONAttrs         map[string]any // Values will be JSON-serialized and merged into Attrs
	StatusCode        codes.Code
	StatusDescription string
	Events            []Event
	SpanKind          oteltrace.SpanKind
	TimeRange         TimeRange // Optional: if non-zero, validates span timing is within this range
}

func NewSpan(t *testing.T, name string, opts TestSpan) Span {
	t.Helper()

	// Merge regular attrs and JSON attrs
	srcAttrs := make(map[string]any)
	// Add regular attributes
	for k, v := range opts.Attrs {
		srcAttrs[k] = v
	}

	// Add JSON attributes (serialize values to JSON strings)
	for k, v := range opts.JSONAttrs {
		js, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("failed to marshal JSONAttrs[%q]: %v", k, err)
		}
		srcAttrs[k] = string(js)
	}

	attrs := make([]attr.KeyValue, 0, len(srcAttrs))
	for k, v := range srcAttrs {
		attrs = append(attrs, convertToAttribute(k, v))
	}

	// Convert events
	events := make([]sdktrace.Event, 0, len(opts.Events))
	for _, e := range opts.Events {
		eventAttrs := make([]attr.KeyValue, 0, len(e.Attrs))
		for k, v := range e.Attrs {
			eventAttrs = append(eventAttrs, convertToAttribute(k, v))
		}
		events = append(events, sdktrace.Event{
			Name:       e.Name,
			Attributes: eventAttrs,
			Time:       time.Now(),
		})
	}

	spanKind := opts.SpanKind
	if spanKind == 0 { // SpanKindUnspecified
		spanKind = oteltrace.SpanKindInternal
	}

	return Span{
		t: t,
		Stub: tracetest.SpanStub{
			Name:       name,
			Attributes: attrs,
			Events:     events,
			SpanKind:   spanKind,
			Status: sdktrace.Status{
				Code:        opts.StatusCode,
				Description: opts.StatusDescription,
			},
		},
	}
}

// convertToAttribute converts various Go types to OpenTelemetry attributes
func convertToAttribute(key string, value any) attr.KeyValue {
	switch v := value.(type) {
	case string:
		return attr.String(key, v)
	case int:
		return attr.Int64(key, int64(v))
	case int64:
		return attr.Int64(key, v)
	case float64:
		return attr.Float64(key, v)
	case bool:
		return attr.Bool(key, v)
	case []string:
		return attr.StringSlice(key, v)
	case []int:
		int64s := make([]int64, len(v))
		for i, val := range v {
			int64s[i] = int64(val)
		}
		return attr.Int64Slice(key, int64s)
	case []int64:
		return attr.Int64Slice(key, v)
	case []float64:
		return attr.Float64Slice(key, v)
	case []bool:
		return attr.BoolSlice(key, v)
	default:
		// For complex types, JSON marshal them
		if jsonBytes, err := json.Marshal(v); err == nil {
			return attr.String(key, string(jsonBytes))
		}
		return attr.String(key, fmt.Sprintf("%v", v))
	}
}

// AssertEqual compares this span with an expected TestSpan. This test ignores
// non-deterministic fields like timestamps and span IDs and only compares the
// meaningful fields like name, status, attributes, and events.
func (s *Span) AssertEqual(expected TestSpan) {
	s.t.Helper()
	expectedSpan := NewSpan(s.t, expected.Name, expected)
	assertSpanStubEqual(s.t, expectedSpan.Stub, s.Stub)

	// Check time range if provided
	if !expected.TimeRange.IsZero() {
		s.AssertInTimeRange(expected.TimeRange)
	}
}

func assertSpanStubEqual(t testingT, s1, s2 tracetest.SpanStub) {
	t.Helper()

	if s1.Name != s2.Name {
		t.Errorf("span name mismatch: expected %q, got %q", s1.Name, s2.Name)
	}

	if s1.Status != s2.Status {
		t.Errorf("span status mismatch: expected %+v, got %+v", s1.Status, s2.Status)
	}

	if s1.SpanKind != s2.SpanKind {
		t.Errorf("span kind mismatch: expected %v, got %v", s1.SpanKind, s2.SpanKind)
	}

	assertAttrsEqual(t, s1.Attributes, s2.Attributes, "")

	for i, event1 := range s1.Events {
		if i < len(s2.Events) {
			event2 := s2.Events[i]
			if event1.Name != event2.Name {
				t.Errorf("event[%d] name mismatch: expected %q, got %q", i, event1.Name, event2.Name)
			}
			assertAttrsEqual(t, event1.Attributes, event2.Attributes, fmt.Sprintf("event[%d] ", i))
		}
	}
	// Compare events one by one
	if len(s1.Events) != len(s2.Events) {
		t.Errorf("number of events mismatch: expected %d, got %d", len(s1.Events), len(s2.Events))
		return
	}

}

func assertAttrsEqual(t testingT, attrs1, attrs2 []attr.KeyValue, prefix string) {
	t.Helper()

	// Create maps for easier attribute lookup
	attrMap1 := make(map[attr.Key]attr.Value)
	for _, a := range attrs1 {
		attrMap1[a.Key] = a.Value
	}

	attrMap2 := make(map[attr.Key]attr.Value)
	for _, a := range attrs2 {
		attrMap2[a.Key] = a.Value
	}

	// Check for missing expected attributes
	for key, val1 := range attrMap1 {
		if val2, exists := attrMap2[key]; !exists {
			t.Errorf("%smissing expected attribute %s", prefix, string(key))
		} else if val1 != val2 {
			t.Errorf("%sattribute %s mismatch: expected %v, got %v", prefix, string(key), val1, val2)
		}
	}

	// Check for unexpected attributes
	for key := range attrMap2 {
		if _, exists := attrMap1[key]; !exists {
			t.Errorf("%shas unexpected attribute %s", prefix, string(key))
		}
	}
}
