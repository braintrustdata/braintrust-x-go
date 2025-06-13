package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
	"github.com/braintrust/braintrust-x-go/braintrust/internal"
)

func setUp(t *testing.T, opts ...sdktrace.TracerProviderOption) (exporter *tracetest.InMemoryExporter, teardown func()) {
	internal.FailTestsOnWarnings(t)

	// setup otel to be fully synchronous
	exporter = tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	opts = append(opts,
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(processor), // flushes immediately
	)

	tp := sdktrace.NewTracerProvider(opts...)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	teardown = func() {
		diag.ClearLogger()
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error shutting down tracer provider: %v", err)
		}
		otel.SetTracerProvider(original)
	}

	return
}

func TestSpanProcessor(t *testing.T) {

	processor := NewSpanProcessor(Project{id: "12345"})
	exporter, teardown := setUp(t, sdktrace.WithSpanProcessor(processor))
	defer teardown()

	assert, require := assert.New(t), require.New(t)
	assert.NotNil(exporter)
	require.NotNil(exporter)

	tracer := otel.GetTracerProvider().Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	span := getOneSpan(t, exporter)
	ok, attr := getAttr(span, PARENT_ATTR)
	require.True(ok)
	assert.Equal(attr.AsString(), "project_id:12345")

	// Assert we use the parent from the context if it is set.
	ctx := context.Background()
	ctx = SetParent(ctx, Project{id: "67890"})
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	span = getOneSpan(t, exporter)
	ok, attr = getAttr(span, PARENT_ATTR)
	require.True(ok)
	assert.Equal(attr.AsString(), "project_id:67890")

	// assert that if a span already has a parent, it is not overridden
	ctx = context.Background()
	ctx = SetParent(ctx, Project{id: "77777"})
	_, span4 := tracer.Start(ctx, "test", trace.WithAttributes(attribute.String(PARENT_ATTR, "project_id:88888")))
	span4.End()
	span = getOneSpan(t, exporter)
	ok, attr = getAttr(span, PARENT_ATTR)
	require.True(ok)
	assert.Equal(attr.AsString(), "project_id:88888")
}

func getOneSpan(t *testing.T, exporter *tracetest.InMemoryExporter) tracetest.SpanStub {
	spans := flushSpans(exporter)
	require.Len(t, spans, 1)
	return spans[0]
}

func flushSpans(exporter *tracetest.InMemoryExporter) []tracetest.SpanStub {
	spans := exporter.GetSpans()
	exporter.Reset()
	return spans
}

func getAttr(span tracetest.SpanStub, key string) (bool, attribute.Value) {
	attrs := span.Attributes
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return true, attr.Value
		}
	}
	return false, attribute.Value{}
}
