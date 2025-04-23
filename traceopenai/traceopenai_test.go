package traceopenai

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const TEST_MODEL = openai.ChatModelGPT4oMini

// setUpTest is a helper function that sets up a new tracer provider for each test.
// It returns an openai client, an exporter, and a teardown function.
func setUpTest(t *testing.T) (openai.Client, *tracetest.InMemoryExporter, func()) {
	// setup otel to be fully synchronous
	exporter := tracetest.NewInMemoryExporter()
	processor := trace.NewSimpleSpanProcessor(exporter)

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSpanProcessor(processor), // flushes immediately
	)

	// Get the current global provider before overwriting it
	original := otel.GetTracerProvider()

	// Set the new provider
	otel.SetTracerProvider(tp)

	teardown := func() {
		// Ensure all spans are flushed
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error shutting down tracer provider: %v", err)
		}

		// Restore the original provider
		otel.SetTracerProvider(original)
	}

	// set up traced openai client
	client := openai.NewClient(
		option.WithMiddleware(Middleware),
	)

	return client, exporter, teardown
}

func TestError(t *testing.T) {
	_, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)

	errorware := func(req *http.Request, next NextMiddleware) (*http.Response, error) {
		return nil, errors.New("ye-olde-test-error")
	}

	client := openai.NewClient(
		option.WithMaxRetries(0), // don't retry errors
		option.WithMiddleware(Middleware),
		option.WithMiddleware(errorware),
	)

	resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("hai")},
		Model: TEST_MODEL,
	})
	assert.Error(err)
	assert.Nil(resp)

	spans := flushSpans(exporter)
	assert.Len(spans, 1)
	span := spans[0]

	assert.Equal("openai.responses.create", span.Name)
	assert.Equal(codes.Error, span.Status.Code)
	assert.Contains(span.Status.Description, "ye-olde-test-error")

	events := span.Events
	assert.Len(events, 1)

	// Find the exception.message attribute that contains our error message
	var errMessage string
	for _, attr := range events[0].Attributes {
		if attr.Key == "exception.message" {
			errMessage = attr.Value.AsString()
			break
		}
	}
	assert.Contains(errMessage, "ye-olde-test-error")
}

func TestOpenAIResponsesRequiredParamsOnly(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
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
	assert.Equal(span.Name, "openai.responses.create")
	assert.Equal(codes.Unset, span.Status.Code)
	assert.Equal("", span.Status.Description)

	valsByKey := toValuesByKey(span.Attributes)
	assert.Contains(valsByKey["model"].AsString(), TEST_MODEL)
	assert.Equal("openai", valsByKey["provider"].AsString())
	assert.Equal("Hello, world!", valsByKey["input"].AsString())
}

func TestOpenAIResponsesAllFields(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
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
	assert.Equal(span.Name, "openai.responses.create")

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
	// Wait a moment for spans to be exported
	// Get spans without resetting the exporter
	spans := exporter.GetSpans()
	exporter.Reset()
	return spans
}

func TestOTelSetup(t *testing.T) {
	_, exporter, teardown := setUpTest(t)
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
