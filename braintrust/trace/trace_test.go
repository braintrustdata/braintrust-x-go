package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
)

// withSpanProcessor is a test-only option for setting a custom span processor
func withSpanProcessor(processor sdktrace.SpanProcessor) braintrust.Option {
	return func(c *braintrust.Config) {
		c.SpanProcessor = processor
	}
}

func newProjectIDParent(projectID string) Parent {
	return Parent{Type: ParentTypeProjectID, ID: projectID}
}

// Helper functions for testing
func flushOne(t *testing.T, exporter *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	spans := exporter.GetSpans()
	exporter.Reset()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

func attrEquals(span tracetest.SpanStub, key, expectedValue string) bool {
	for _, attr := range span.Attributes {
		if string(attr.Key) == key && attr.Value.AsString() == expectedValue {
			return true
		}
	}
	return false
}

func assertAttrEquals(t *testing.T, span tracetest.SpanStub, key, expectedValue string) {
	t.Helper()
	assert.True(t, attrEquals(span, key, expectedValue))
}

func TestSpanProcessor(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("12345"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:12345")

	// Assert we use the parent from the context if it is set.
	ctx := context.Background()
	ctx = SetParent(ctx, newProjectIDParent("67890"))
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	_ = tp.ForceFlush(context.Background())
	span = flushOne(t, exporter)
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:67890")

	// assert that if a span already has a parent, it is not overridden
	ctx = context.Background()
	ctx = SetParent(ctx, newProjectIDParent("77777"))
	_, span4 := tracer.Start(ctx, "test", trace.WithAttributes(attribute.String(ParentOtelAttrKey, "project_id:88888")))
	span4.End()
	_ = tp.ForceFlush(context.Background())
	span = flushOne(t, exporter)
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:88888")
}
func TestSpanProcessorNoDefaultProjectID(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Don't set a default project - should use the fallback
	err := Enable(tp,
		braintrust.WithDefaultProject(""), // Empty project name
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the fallback default parent
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_name:go-otel-default-project")
}

func TestSpanProcessorWithDefaultProjectName(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProject("12345"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_name:12345")
}

func TestEnableErrors(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	err := Enable(tp, braintrust.WithAPIURL("invalid-url"))
	assert.Error(t, err)
}

func TestEnableOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []braintrust.Option
		wantAttr string
	}{
		{"project id", []braintrust.Option{braintrust.WithDefaultProjectID("test-123")}, "project_id:test-123"},
		{"project name", []braintrust.Option{braintrust.WithDefaultProject("test-name")}, "project_name:test-name"},
		{"no project", []braintrust.Option{}, "project_name:default-go-project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			exporter := tracetest.NewInMemoryExporter()
			processor := sdktrace.NewSimpleSpanProcessor(exporter)
			opts := append(tt.opts, withSpanProcessor(processor))

			err := Enable(tp, opts...)
			assert.NoError(t, err)

			_, span := tp.Tracer("test").Start(context.Background(), "test-span")
			span.End()
			err = tp.ForceFlush(context.Background())
			assert.NoError(t, err)

			span1 := flushOne(t, exporter)
			assertAttrEquals(t, span1, ParentOtelAttrKey, tt.wantAttr)
		})
	}
}

func TestEnableWithExistingProcessors(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	m1 := tracetest.NewInMemoryExporter()

	// Add one "existing" processor (e.g. a customer's APM processor)
	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(m1))

	// Add Braintrust's span processor
	m2 := tracetest.NewInMemoryExporter()
	processor2 := sdktrace.NewSimpleSpanProcessor(m2)
	err := Enable(tp,
		braintrust.WithDefaultProjectID("test-project-existing"),
		withSpanProcessor(processor2),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span-existing")
	span.End()

	err = tp.ForceFlush(context.Background())
	assert.NoError(err)

	span1, span2 := flushOne(t, m1), flushOne(t, m2)
	assert.Equal("test-span-existing", span1.Name)
	assert.Equal("test-span-existing", span2.Name)
	assertAttrEquals(t, span1, ParentOtelAttrKey, "project_id:test-project-existing")
	assertAttrEquals(t, span2, ParentOtelAttrKey, "project_id:test-project-existing")
}

func TestQuickstart(t *testing.T) {
	original := otel.GetTracerProvider()
	defer otel.SetTracerProvider(original)

	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	teardown, err := Quickstart(
		braintrust.WithDefaultProject("test-quickstart"),
		withSpanProcessor(processor),
	)
	assert.NoError(t, err)
	assert.NotNil(t, teardown)

	tp := otel.GetTracerProvider()
	assert.NotEqual(t, original, tp)

	// Test that we can create spans with the global tracer
	tracer := otel.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	capturedSpan := spans[0]
	assert.Equal(t, "test-span", capturedSpan.Name)
	assertAttrEquals(t, capturedSpan, ParentOtelAttrKey, "project_name:test-quickstart")

	teardown()
}
