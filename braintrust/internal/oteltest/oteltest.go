package oteltest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

// Summary returns the core data of the span, including attributes, events, and status.
// It omits non-deterministic fields such as timestamps and span IDs. If two spans trace
// the same code path with identical data, their Summary outputs should be equal,
// even if the spans themselves are not.
func (s *Span) Summary() map[string]any {
	return map[string]any{
		"name":       s.Stub.Name,
		"spanKind":   s.Stub.SpanKind.String(),
		"attributes": convertAttributes(s.t, s.Stub.Attributes),
		"events":     convertEvents(s.t, s.Stub.Events),
		"status": map[string]any{
			"code":        s.Stub.Status.Code.String(),
			"description": s.Stub.Status.Description,
		},
	}
}

// Snapshot returns a JSON string containing the span's summary. Read the docs on
// Summary() for more information.
func (s *Span) Snapshot() string {
	s.t.Helper()
	jsonBytes, err := json.MarshalIndent(s.Summary(), "", "  ")
	if err != nil {
		s.t.Fatalf("Failed to marshal span snapshot: %v", err)
	}
	return string(jsonBytes)
}

func convertAttributes(t *testing.T, attrs []attr.KeyValue) map[string]interface{} {
	t.Helper()
	result := make(map[string]interface{})
	for _, a := range attrs {
		if a.Key == "" {
			t.Fatalf("Empty attribute key found")
		}
		result[string(a.Key)] = convertAttributeValue(t, a.Value)
	}
	return result
}

func convertAttributeValue(t *testing.T, value attr.Value) interface{} {
	t.Helper()
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
	case attr.INVALID:
		t.Fatalf("Invalid attribute value encountered")
		return nil
	default:
		t.Fatalf("Unsupported attribute type: %v", value.Type())
		return nil
	}
}

func convertEvents(t *testing.T, events []sdktrace.Event) []map[string]interface{} {
	t.Helper()
	result := make([]map[string]interface{}, len(events))
	for i, event := range events {
		result[i] = map[string]interface{}{
			"name":       event.Name,
			"attributes": convertAttributes(t, event.Attributes),
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

// Event is a summary of an otel event.
type Event struct {
	Name  string
	Attrs map[string]any
}

// TestSpan is a summary of an otel span that is a little bit easier
// to write in tests. It only contains "deterministic" fields that
// we want to compare in tests (e.g it skips timestamps, span IDs, etc)
type TestSpan struct {
	Name              string
	Attrs             map[string]any
	StatusCode        string
	StatusDescription string
	Events            []Event
	TimeRange         TimeRange // Optional: if non-zero, validates span timing is within this range
}

func NewSpan(name string, opts TestSpan) *Span {
	attrs := make([]attr.KeyValue, 0, len(opts.Attrs))
	for k, v := range opts.Attrs {
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

	// Parse status code - default to Unset if empty
	var statusCode codes.Code
	if opts.StatusCode == "" {
		statusCode = codes.Unset
	} else {
		switch opts.StatusCode {
		case "OK":
			statusCode = codes.Ok
		case "ERROR":
			statusCode = codes.Error
		case "Unset":
			statusCode = codes.Unset
		default:
			if code, err := strconv.Atoi(opts.StatusCode); err == nil {
				statusCode = codes.Code(code)
			} else {
				statusCode = codes.Unset
			}
		}
	}

	return &Span{
		Stub: tracetest.SpanStub{
			Name:       name,
			Attributes: attrs,
			Events:     events,
			Status: sdktrace.Status{
				Code:        statusCode,
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

// AssertEqual compares this span with an expected TestSpan using individual assertions for clear error messages
func (s *Span) AssertEqual(expected TestSpan) {
	s.t.Helper()

	// Compare name
	assert.Equal(s.t, expected.Name, s.Stub.Name, "span name")

	// Parse and compare status code
	var expectedStatusCode codes.Code
	if expected.StatusCode == "" {
		expectedStatusCode = codes.Unset
	} else {
		switch expected.StatusCode {
		case "OK":
			expectedStatusCode = codes.Ok
		case "ERROR":
			expectedStatusCode = codes.Error
		case "Unset":
			expectedStatusCode = codes.Unset
		default:
			if code, err := strconv.Atoi(expected.StatusCode); err == nil {
				expectedStatusCode = codes.Code(code)
			} else {
				expectedStatusCode = codes.Unset
			}
		}
	}

	assert.Equal(s.t, expectedStatusCode, s.Stub.Status.Code, "span status code")
	assert.Equal(s.t, expected.StatusDescription, s.Stub.Status.Description, "span status description")

	// Compare attributes individually
	actualAttrs := convertAttributes(s.t, s.Stub.Attributes)

	for key, expectedVal := range expected.Attrs {
		assert.Contains(s.t, actualAttrs, key, "missing attribute %q", key)
		if actualVal, exists := actualAttrs[key]; exists {
			assert.Equal(s.t, expectedVal, actualVal, "attribute %q", key)
		}
	}

	for key := range actualAttrs {
		assert.Contains(s.t, expected.Attrs, key, "unexpected attribute %q", key)
	}

	// Compare events individually
	actualEvents := convertEvents(s.t, s.Stub.Events)

	assert.Equal(s.t, len(expected.Events), len(actualEvents), "number of events")

	for i, expectedEvent := range expected.Events {
		if i < len(actualEvents) {
			actualEvent := actualEvents[i]
			assert.Equal(s.t, expectedEvent.Name, actualEvent["name"], "event[%d] name", i)

			actualEventAttrs := actualEvent["attributes"].(map[string]interface{})

			for key, expectedVal := range expectedEvent.Attrs {
				assert.Contains(s.t, actualEventAttrs, key, "event[%d] missing attribute %q", i, key)
				if actualVal, exists := actualEventAttrs[key]; exists {
					assert.Equal(s.t, expectedVal, actualVal, "event[%d] attribute %q", i, key)
				}
			}

			for key := range actualEventAttrs {
				assert.Contains(s.t, expectedEvent.Attrs, key, "event[%d] unexpected attribute %q", i, key)
			}
		}
	}

	// Validate time range if provided
	if !expected.TimeRange.Start.IsZero() && !expected.TimeRange.End.IsZero() {
		assert.True(s.t, expected.TimeRange.Start.Before(s.Stub.StartTime), "span start time should be after TimeRange.Start")
		assert.True(s.t, expected.TimeRange.End.After(s.Stub.EndTime), "span end time should be before TimeRange.End")
		assert.True(s.t, s.Stub.StartTime.Before(s.Stub.EndTime), "span start time should be before end time")
	}
}
