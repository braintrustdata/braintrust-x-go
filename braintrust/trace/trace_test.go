package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/oteltest"
)

// withSpanExporter is a test-only option for setting a custom span exporter
func withSpanExporter(exporter sdktrace.SpanExporter) braintrust.Option {
	return func(c *braintrust.Config) {
		c.SpanExporter = exporter
	}
}

func newProjectIDParent(projectID string) Parent {
	return Parent{Type: ParentTypeProjectID, ID: projectID}
}

func newProjectNameParent(projectName string) Parent {
	return Parent{Type: ParentTypeProject, ID: projectName}
}

func TestSpanProcessor(t *testing.T) {
	assert := assert.New(t)

	processor := NewSpanProcessor(WithDefaultParent(newProjectIDParent("12345")))
	tracer, exporter := oteltest.Setup(t, sdktrace.WithSpanProcessor(processor))

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	span := exporter.FlushOne()

	assert.Equal(span.Name(), "test")
	span.AssertAttrEquals(ParentOtelAttrKey, "project_id:12345")

	// Assert we use the parent from the context if it is set.
	ctx := context.Background()
	ctx = SetParent(ctx, newProjectIDParent("67890"))
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	span = exporter.FlushOne()
	span.AssertAttrEquals(ParentOtelAttrKey, "project_id:67890")

	// assert that if a span already has a parent, it is not overridden
	ctx = context.Background()
	ctx = SetParent(ctx, newProjectIDParent("77777"))
	_, span4 := tracer.Start(ctx, "test", trace.WithAttributes(attribute.String(ParentOtelAttrKey, "project_id:88888")))
	span4.End()
	span = exporter.FlushOne()
	span.AssertAttrEquals(ParentOtelAttrKey, "project_id:88888")
}
func TestSpanProcessorNoDefaultProjectID(t *testing.T) {
	assert := assert.New(t)

	processor := NewSpanProcessor()
	tracer, exporter := oteltest.Setup(t, sdktrace.WithSpanProcessor(processor))

	// Assert we don't set a parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	span := exporter.FlushOne()

	assert.Equal(span.Name(), "test")
	assert.False(span.HasAttr(ParentOtelAttrKey))
}

func TestSpanProcessorWithDefaultProjectName(t *testing.T) {
	assert := assert.New(t)

	processor := NewSpanProcessor(WithDefaultParent(newProjectNameParent("12345")))
	tracer, exporter := oteltest.Setup(t, sdktrace.WithSpanProcessor(processor))

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	span := exporter.FlushOne()

	assert.Equal(span.Name(), "test")
	span.AssertAttrEquals(ParentOtelAttrKey, "project_name:12345")
}

func TestEnable(t *testing.T) {
	assert := assert.New(t)

	// Test that Enable fails with invalid URL format
	tp := sdktrace.NewTracerProvider()

	// Test with malformed URL (missing protocol separator)
	err := Enable(tp,
		braintrust.WithAPIKey("test-api-key"),
		braintrust.WithAPIURL("invalid-url"),
	)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid url")

	// Test that Enable successfully configures tracer provider with memory exporter
	tp2 := sdktrace.NewTracerProvider()
	memoryExporter := tracetest.NewInMemoryExporter()
	err = Enable(tp2,
		braintrust.WithDefaultProjectID("test-project-123"),
		withSpanExporter(memoryExporter),
	)
	assert.NoError(err)

	// Create a span and verify it gets the correct attributes
	tracer := tp2.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	// Force flush and verify the span was captured with correct attributes
	err = tp2.ForceFlush(context.Background())
	assert.NoError(err)

	spans := memoryExporter.GetSpans()
	assert.Len(spans, 1)
	if len(spans) > 0 {
		capturedSpan := spans[0]
		assert.Equal("test-span", capturedSpan.Name)
		// Verify the Braintrust span processor added the parent attribute
		found := false
		for _, attr := range capturedSpan.Attributes {
			if string(attr.Key) == ParentOtelAttrKey && attr.Value.AsString() == "project_id:test-project-123" {
				found = true
				break
			}
		}
		assert.True(found)
	}

	// Test Enable with project name instead of project ID
	tp3 := sdktrace.NewTracerProvider()
	err = Enable(tp3, braintrust.WithDefaultProject("test-project-name"))
	assert.NoError(err)

	// Test Enable without any default project (should still work)
	tp4 := sdktrace.NewTracerProvider()
	err = Enable(tp4)
	assert.NoError(err)
}

func TestEnableWithExistingProcessors(t *testing.T) {
	assert := assert.New(t)

	// Create a tracer provider and register two existing processors
	tp := sdktrace.NewTracerProvider()

	m1 := oteltest.NewExporter(t)
	m2 := oteltest.NewExporter(t)

	// Add one "existing" processor (e.g. a customer's APM processor)
	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(m1.InMemoryExporter()))

	// Now add Braintrusts
	err := Enable(tp,
		braintrust.WithDefaultProjectID("test-project-existing"),
		withSpanExporter(m2.InMemoryExporter()),
	)
	assert.NoError(err)

	// Create a span - it should go to all three processors
	tracer := tp.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span-existing")
	span.End()

	// Force flush to ensure all processors receive the span
	err = tp.ForceFlush(context.Background())
	assert.NoError(err)

	// Verify all three exporters received the span
	span1, span2 := m1.FlushOne(), m2.FlushOne()

	// Verify all spans have the same name
	assert.Equal("test-span-existing", span1.Name())
	assert.Equal("test-span-existing", span2.Name())

	// Verify all spans have the parent attribute
	// (all processors get the same span data with Braintrust attributes)
	span1.AssertAttrEquals(ParentOtelAttrKey, "project_id:test-project-existing")
	span2.AssertAttrEquals(ParentOtelAttrKey, "project_id:test-project-existing")
}
