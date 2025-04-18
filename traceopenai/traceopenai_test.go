package traceopenai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setUp() (*tracetest.InMemoryExporter, func()) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter), // flushes immediately
	)
	otel.SetTracerProvider(tp)

	teardown := func() {
		otel.SetTracerProvider(nil)
	}

	return exporter, teardown
}

func TestOTelSetup(t *testing.T) {
	exporter, teardown := setUp()
	defer teardown()

	assert := assert.New(t)

	spans := flush(exporter)
	assert.Empty(spans)

	tracer := otel.Tracer("test")
	_, span := tracer.Start(context.Background(), "test")
	span.End()

	spans = flush(exporter)
	assert.NotEmpty(spans)

	spans = flush(exporter)
	assert.Empty(spans)
}

func flush(exporter *tracetest.InMemoryExporter) []tracetest.SpanStub {
	spans := exporter.GetSpans()
	exporter.Reset()
	return spans
}
