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

func TestEnableErrors(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	memoryExporter := tracetest.NewInMemoryExporter()

	err := Enable(tp, braintrust.WithAPIURL("invalid-url"), withSpanExporter(memoryExporter))
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
			exporter := oteltest.NewExporter(t)
			opts := append(tt.opts, withSpanExporter(exporter.InMemoryExporter()))

			err := Enable(tp, opts...)
			assert.NoError(t, err)

			_, span := tp.Tracer("test").Start(context.Background(), "test-span")
			span.End()
			tp.ForceFlush(context.Background())

			span1 := exporter.FlushOne()
			span1.AssertAttrEquals(ParentOtelAttrKey, tt.wantAttr)
		})
	}
}

func TestEnableWithExistingProcessors(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	m1 := oteltest.NewExporter(t)
	m2 := oteltest.NewExporter(t)

	// Add one "existing" processor (e.g. a customer's APM processor)
	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(m1.InMemoryExporter()))

	// Add Braintrust's span processor
	err := Enable(tp,
		braintrust.WithDefaultProjectID("test-project-existing"),
		withSpanExporter(m2.InMemoryExporter()),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span-existing")
	span.End()

	err = tp.ForceFlush(context.Background())
	assert.NoError(err)

	span1, span2 := m1.FlushOne(), m2.FlushOne()
	assert.Equal("test-span-existing", span1.Name())
	assert.Equal("test-span-existing", span2.Name())
	span1.AssertAttrEquals(ParentOtelAttrKey, "project_id:test-project-existing")
	span2.AssertAttrEquals(ParentOtelAttrKey, "project_id:test-project-existing")
}
