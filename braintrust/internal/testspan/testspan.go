package testspan

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestSpan is a small wrapper around OTel SpanStubs with Braintrust encoded
// data. It has some helper functions that will automatically fail tests
// if you try to read missing data, or it can't decode JSON fields etc. It cuts
// out some boiler plate in testing code.
type TestSpan struct {
	t    *testing.T
	Stub tracetest.SpanStub
}

// New teturns a new test span.
func New(t *testing.T, stub tracetest.SpanStub) *TestSpan {
	return &TestSpan{
		t:    t,
		Stub: stub,
	}
}

// String returns a pretty print of the span.
func (s *TestSpan) String() string {
	// Create a nicely formatted string representation of the span
	var result string
	result += "TestSpan:\n"
	result += "  Name: " + s.Stub.Name + "\n"
	result += "  Start Time: " + s.Stub.StartTime.String() + "\n"
	result += "  End Time: " + s.Stub.EndTime.String() + "\n"

	result += "  Attributes:\n"
	for _, attr := range s.Stub.Attributes {
		key := string(attr.Key)
		val := attr.Value.AsString()

		// Try to pretty print JSON attributes
		if isJSON(val) {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, []byte(val), "    ", "  "); err == nil {
				val = "\n" + prettyJSON.String()
			}
		}

		result += "    " + key + ": " + val + "\n"
	}

	return result
}

func (s *TestSpan) AssertNameIs(n string) {
	require.Equal(s.t, n, s.Stub.Name)
}

func (s *TestSpan) AssertTimingIsValid(start, end time.Time) {
	require.True(s.t, s.Stub.StartTime.After(start))
	require.True(s.t, s.Stub.EndTime.After(s.Stub.StartTime))
	require.True(s.t, s.Stub.EndTime.Before(end))
}

// Attr returns the Attr with the given key if it exists.
func (s *TestSpan) Attr(key string) (bool, attr.Value) {
	for _, a := range s.Stub.Attributes {
		// MATT: fail if more than one?
		if string(a.Key) == key {
			return true, a.Value
		}
	}
	return false, attr.Value{}
}

func (s *TestSpan) AttrString(key string) string {
	found, val := s.Attr(key)
	require.True(s.t, found, "attribute %s not found", key)
	require.True(s.t, val.Type() == attr.STRING)
	return val.AsString()
}

// AttrMust returns the value of the attribute with the given key and fails the test
// if not found.
func (s *TestSpan) AttrMust(key string) attr.Value {
	found, val := s.Attr(key)
	require.True(s.t, found, "attribute %s not found", key)
	return val
}

func (s *TestSpan) Input() any {
	var input any
	s.unmarshal("braintrust.input", &input)
	return input
}

func (s *TestSpan) Output() any {
	var output any
	s.unmarshal("braintrust.output", &output)
	return output
}

func (s *TestSpan) Metadata() map[string]any {
	var m map[string]any
	s.unmarshal("braintrust.metadata", &m)
	return m
}

func (s *TestSpan) Metrics() map[string]float64 {
	var m map[string]float64
	s.unmarshal("braintrust.metrics", &m)
	return m
}

func (s *TestSpan) unmarshal(key string, into any) {
	raw := s.AttrMust(key)
	require.True(s.t, raw.Type() == attr.STRING)
	err := json.Unmarshal([]byte(raw.AsString()), into)
	require.NoError(s.t, err)
}

func isJSON(s string) bool {
	var js any
	return json.Unmarshal([]byte(s), &js) == nil
}
