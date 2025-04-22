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
	assert.Contains(valsByKey["model"].AsString(), TEST_MODEL)
	assert.Equal("openai", valsByKey["provider"].AsString())
	assert.Equal("Hello, world!", valsByKey["input"].AsString())
}

func TestOpenAIResponsesAllFields(t *testing.T) {
	client, exporter, teardown := setUpTracedClient()
	defer teardown()
	assert := assert.New(t)

	input := responses.ResponseNewParamsInputUnion{OfString: openai.String("what is 13+4?")}

	// Test with string output
	params := responses.ResponseNewParams{
		Input: input,
		Model: TEST_MODEL,
	}

	resp, err := client.Responses.New(context.Background(), params)
	assert.NoError(err)
	assert.NotNil(resp)

	// Wait for spans to be exported
	spans := flushSpans(exporter)
	if len(spans) == 0 {
		t.Fatal("No spans were generated")
	}
	span := spans[0]
	assert.Equal(span.Name, "openai.chat.completion")

	valsByKey := toValuesByKey(span.Attributes)

	// Required fields
	assert.Equal("openai", valsByKey["provider"].AsString())
	assert.Equal(resp.ID, valsByKey["id"].AsString())
	assert.Contains(valsByKey["model"].AsString(), TEST_MODEL)

	// Output field
	outputText := resp.OutputText()
	assert.Contains(outputText, "17")

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
