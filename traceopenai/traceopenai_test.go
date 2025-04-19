package traceopenai

import (
	"context"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const TEST_MODEL = openai.ChatModelGPT4oMini

func setUpTracedClient() (openai.Client, *tracetest.InMemoryExporter, func()) {
	// setup otel
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter), // flushes immediately
	)
	otel.SetTracerProvider(tp)
	teardown := func() {
		otel.SetTracerProvider(nil)
	}
	// set up traced openai client
	client := openai.NewClient(
		option.WithMiddleware(Middleware),
	)

	return client, exporter, teardown
}

func TestOpenAIResponsesRequiredParamsOnly(t *testing.T) {
	client, exporter, teardown := setUpTracedClient()
	defer teardown()
	assert := assert.New(t)

	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("Hello, world!")},
		Model: TEST_MODEL,
	}

	resp, err := client.Responses.New(context.Background(), params)
	assert.NoError(err)
	assert.NotNil(resp)

	spans := flushSpans(exporter)
	assert.Len(spans, 1)
	span := spans[0]
	assert.Equal(span.Name, "openai.chat.completion")

	valsByKey := toValuesByKey(span.Attributes)
	assert.Equal(valsByKey["model"].AsString(), "gpt-4o-mini")
	assert.Equal(valsByKey["provider"].AsString(), "openai")
	assert.Equal(valsByKey["input"].AsString(), "Hello, world!")
}

func toValuesByKey(attrs []attribute.KeyValue) map[string]attribute.Value {
	attrsByKey := make(map[string]attribute.Value)
	for _, attr := range attrs {
		attrsByKey[string(attr.Key)] = attr.Value
	}
	return attrsByKey
}

func flushSpans(exporter *tracetest.InMemoryExporter) []tracetest.SpanStub {
	spans := exporter.GetSpans()
	exporter.Reset()
	return spans
}

func TestOTelSetup(t *testing.T) {
	_, exporter, teardown := setUpTracedClient()
	defer teardown()
	assert := assert.New(t)

	// crudely check we can create and test spans
	spans := flushSpans(exporter)
	assert.Empty(spans)

	tracer := otel.Tracer("test")
	_, span := tracer.Start(context.Background(), "test")
	span.End()

	spans = flushSpans(exporter)
	assert.NotEmpty(spans)
	spans = flushSpans(exporter)
	assert.Empty(spans)
}
