package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/oteltest"
)

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
