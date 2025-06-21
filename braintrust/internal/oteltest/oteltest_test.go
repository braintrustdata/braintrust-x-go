package oteltest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

	// Use assert.JSONEq for robust JSON comparison
	assert.JSONEq(t, expected, snapshot)
}

func TestAssertSpanStubEqual_Success(t *testing.T) {
	span1 := createTestSpanStub("test-span", codes.Ok, "success")
	span2 := createTestSpanStub("test-span", codes.Ok, "success")

	// Create a mock to capture any errors
	mock := &mockTesting{}

	// This should pass without errors
	assertSpanStubEqual(mock, span1, span2)

	assert.False(t, mock.failed, "Expected test to pass")
	assert.Empty(t, mock.errorMessage, "Expected no error message")
}

func TestAssertSpanStubEqual_NameMismatch(t *testing.T) {
	span1 := createTestSpanStub("span1", codes.Ok, "success")
	span2 := createTestSpanStub("span2", codes.Ok, "success")

	mock := &mockTesting{}
	assertSpanStubEqual(mock, span1, span2)

	assert.True(t, mock.failed, "Expected test to fail")
	assert.Contains(t, mock.errorMessage, "span name mismatch")
	assert.Contains(t, mock.errorMessage, "expected \"span1\"")
	assert.Contains(t, mock.errorMessage, "got \"span2\"")
}

func TestAssertSpanStubEqual_StatusMismatch(t *testing.T) {
	span1 := createTestSpanStub("test-span", codes.Ok, "success")
	span2 := createTestSpanStub("test-span", codes.Error, "failed")

	mock := &mockTesting{}
	assertSpanStubEqual(mock, span1, span2)

	assert.True(t, mock.failed, "Expected test to fail")
	assert.Contains(t, mock.errorMessage, "span status mismatch")
}

func TestAssertSpanStubEqual_AttributeMismatch(t *testing.T) {
	span1 := createTestSpanStubWithAttrs("test-span", map[string]string{
		"key1": "value1",
		"key2": "value2",
	})
	span2 := createTestSpanStubWithAttrs("test-span", map[string]string{
		"key1": "different_value",
		"key3": "value3",
	})

	mock := &mockTesting{}
	assertSpanStubEqual(mock, span1, span2)

	assert.True(t, mock.failed, "Expected test to fail")
	// Should complain about missing key2, unexpected key3, and different value for key1
	assert.Contains(t, mock.errorMessage, "missing expected attribute key2")
}

func TestAssertSpanStubEqual_EventMismatch(t *testing.T) {
	span1 := createTestSpanStubWithEvents("test-span", []string{"event1", "event2"})
	span2 := createTestSpanStubWithEvents("test-span", []string{"event1"})

	mock := &mockTesting{}
	assertSpanStubEqual(mock, span1, span2)

	assert.True(t, mock.failed, "Expected test to fail")
	assert.Contains(t, mock.errorMessage, "number of events mismatch")
	assert.Contains(t, mock.errorMessage, "expected 2")
	assert.Contains(t, mock.errorMessage, "got 1")
}

// Helper functions for creating test spans
func createTestSpanStub(name string, statusCode codes.Code, statusDesc string) tracetest.SpanStub {
	return tracetest.SpanStub{
		Name:     name,
		SpanKind: oteltrace.SpanKindInternal,
		Status: sdktrace.Status{
			Code:        statusCode,
			Description: statusDesc,
		},
	}
}

func createTestSpanStubWithAttrs(name string, attrs map[string]string) tracetest.SpanStub {
	attributes := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		attributes = append(attributes, attribute.String(k, v))
	}

	return tracetest.SpanStub{
		Name:       name,
		SpanKind:   oteltrace.SpanKindInternal,
		Attributes: attributes,
	}
}

func createTestSpanStubWithEvents(name string, eventNames []string) tracetest.SpanStub {
	events := make([]sdktrace.Event, len(eventNames))
	for i, eventName := range eventNames {
		events[i] = sdktrace.Event{
			Name: eventName,
		}
	}

	return tracetest.SpanStub{
		Name:     name,
		SpanKind: oteltrace.SpanKindInternal,
		Events:   events,
	}
}

// mockTesting implements the minimal testingT interface for testing
type mockTesting struct {
	failed       bool
	errorMessage string
}

func (m *mockTesting) Helper() {}

func (m *mockTesting) Errorf(format string, args ...interface{}) {
	m.failed = true
	if m.errorMessage != "" {
		m.errorMessage += "; "
	}
	m.errorMessage += fmt.Sprintf(format, args...)
}

func TestNewSpan_JSONAttrs(t *testing.T) {
	testSpan := TestSpan{
		Name: "test-span",
		Attrs: map[string]any{
			"regular_attr": "regular_value",
		},
		JSONAttrs: map[string]any{
			"json_string": "hello",
			"json_number": 42,
			"json_object": map[string]any{
				"nested": "value",
				"count":  123,
			},
			"json_array": []string{"item1", "item2"},
		},
	}

	span := NewSpan(t, testSpan.Name, testSpan)
	assert.NotNil(t, span)

	// Verify regular attribute
	span.AssertAttrEquals("regular_attr", "regular_value")

	// Verify JSON attributes are serialized correctly
	span.AssertAttrEquals("json_string", `"hello"`)
	span.AssertAttrEquals("json_number", "42")
	span.AssertAttrEquals("json_object", `{"count":123,"nested":"value"}`)
	span.AssertAttrEquals("json_array", `["item1","item2"]`)
}

func TestNewSpan_JSONAttrs_Override(t *testing.T) {
	// Test that JSONAttrs can override regular Attrs
	testSpan := TestSpan{
		Name: "test-span",
		Attrs: map[string]any{
			"key": "regular_value",
		},
		JSONAttrs: map[string]any{
			"key": "json_value",
		},
	}

	span := NewSpan(t, testSpan.Name, testSpan)
	assert.NotNil(t, span)

	// JSONAttrs should override regular Attrs
	span.AssertAttrEquals("key", `"json_value"`)
}
