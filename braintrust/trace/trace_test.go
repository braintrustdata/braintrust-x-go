package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
)

func TestSpanProcessor(t *testing.T) {
	assert := assert.New(t)

	processor := NewSpanProcessor(Project{id: "12345"})
	exporter := oteltest.SetupTracer(t, sdktrace.WithSpanProcessor(processor))

	tracer := otel.GetTracerProvider().Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(t.Context(), "test")
	span1.End()
	span := exporter.FlushOne()

	assert.Equal(span.Name(), "test")
	span.Attr(PARENT_ATTR).AssertEquals("project_id:12345")

	// Assert we use the parent from the context if it is set.

	// Assert we use the parent from the context if it is set.
	ctx := t.Context()
	ctx = SetParent(ctx, Project{id: "67890"})
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	span = exporter.FlushOne()
	span.Attr(PARENT_ATTR).AssertEquals("project_id:67890")

	// assert that if a span already has a parent, it is not overridden
	ctx = t.Context()
	ctx = SetParent(ctx, Project{id: "77777"})
	_, span4 := tracer.Start(ctx, "test", trace.WithAttributes(attribute.String(PARENT_ATTR, "project_id:88888")))
	span4.End()
	span = exporter.FlushOne()
	span.Attr(PARENT_ATTR).AssertEquals("project_id:88888")
}
