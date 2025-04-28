package traceopenai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	teardown := func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error shutting down tracer provider: %v", err)
		}
		otel.SetTracerProvider(original)
	}

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
	var errMsg string
	for _, attr := range events[0].Attributes {
		if attr.Key == "exception.message" {
			errMsg = attr.Value.AsString()
			break
		}
	}
	assert.Contains(errMsg, "ye-olde-test-error")
}

func TestOpenAIResponsesRequiredParams(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)

	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("What is 13+4?")},
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

	// Verify token usage metrics - they must always be present
	assert.Greater(valsByKey["usage.input_tokens"].AsInt64(), int64(0))
	assert.Greater(valsByKey["usage.output_tokens"].AsInt64(), int64(0))
	assert.Greater(valsByKey["usage.total_tokens"].AsInt64(), int64(0))

	// Verify token detail metrics - they must always be present
	assert.GreaterOrEqual(valsByKey["usage.input_tokens_details.cached_tokens"].AsInt64(), int64(0))
	assert.GreaterOrEqual(valsByKey["usage.output_tokens_details.reasoning_tokens"].AsInt64(), int64(0))

	var jsonAttrsMap map[string]interface{}
	rawAttrs := valsByKey["attributes.json.request"].AsString()
	err = json.Unmarshal([]byte(rawAttrs), &jsonAttrsMap)
	assert.NoError(err)
	assert.Contains(rawAttrs, "13+4")

	jsonAttrsMap = make(map[string]interface{})
	rawAttrs = valsByKey["attributes.json.response"].AsString()
	err = json.Unmarshal([]byte(rawAttrs), &jsonAttrsMap)
	assert.NoError(err)
	assert.Contains(rawAttrs, "17")
}

func TestOpenAIResponsesKitchenSink(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)
	require := require.New(t)

	input := responses.ResponseNewParamsInputUnion{OfString: openai.String("what is 13+4?")}

	// Test with string output
	params := responses.ResponseNewParams{
		Input:             input,
		Model:             TEST_MODEL,
		Instructions:      openai.String("Answer the question in a concise manner."),
		MaxOutputTokens:   openai.Int(100),
		ParallelToolCalls: openai.Bool(true),
		Store:             openai.Bool(false),
		Truncation:        responses.ResponseNewParamsTruncationAuto,
		Temperature:       param.Opt[float64]{Value: 0.5},
		TopP:              param.Opt[float64]{Value: 1.0},
		User:              openai.String("test user"),
	}

	resp, err := client.Responses.New(context.Background(), params)
	require.Nil(err)
	require.NotNil(resp)

	// Wait for spans to be exported
	spans := flushSpans(exporter)
	require.Len(spans, 1)

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

	// Instructions field
	assert.Equal("Answer the question in a concise manner.", valsByKey["instructions"].AsString())
	assert.Equal("test user", valsByKey["user"].AsString())
	assert.Equal(0.5, valsByKey["temperature"].AsFloat64())
	assert.Equal(1.0, valsByKey["top_p"].AsFloat64())
	assert.Equal(true, valsByKey["parallel_tool_calls"].AsBool())
	assert.Equal(false, valsByKey["store"].AsBool())
	assert.Equal("auto", valsByKey["truncation"].AsString())
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

func TestOpenAIResponsesWithListInput(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)
	require := require.New(t)

	// Create a list of message inputs
	inputMessages := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfMessage("What is 2+2?", "user"),
		responses.ResponseInputItemParamOfMessage("4", "assistant"),
		responses.ResponseInputItemParamOfMessage("What is 3+125?", "user"),
	}

	// Create the params with list input
	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputMessages,
		},
		Model: TEST_MODEL,
	}

	// Call the API
	resp, err := client.Responses.New(context.Background(), params)
	require.NoError(err)
	require.NotNil(resp)

	// Wait for spans to be exported
	spans := flushSpans(exporter)
	require.Len(spans, 1)
	span := spans[0]

	// Check span attributes
	assert.Equal("openai.responses.create", span.Name)
	assert.Equal(codes.Unset, span.Status.Code)

	valsByKey := toValuesByKey(span.Attributes)
	assert.Contains(valsByKey["model"].AsString(), TEST_MODEL)
	assert.Equal("openai", valsByKey["provider"].AsString())

	assert.Greater(valsByKey["usage.input_tokens"].AsInt64(), int64(0))
	assert.Greater(valsByKey["usage.output_tokens"].AsInt64(), int64(0))
	assert.Greater(valsByKey["usage.total_tokens"].AsInt64(), int64(0))
	assert.GreaterOrEqual(valsByKey["usage.input_tokens_details.cached_tokens"].AsInt64(), int64(0))
	assert.GreaterOrEqual(valsByKey["usage.output_tokens_details.reasoning_tokens"].AsInt64(), int64(0))

	var jsonAttrsMap map[string]interface{}
	rawAttrs := valsByKey["attributes.json.request"].AsString()
	err = json.Unmarshal([]byte(rawAttrs), &jsonAttrsMap)
	assert.NoError(err)
	assert.Contains(rawAttrs, "3+125")

	jsonAttrsMap = make(map[string]interface{})
	rawAttrs = valsByKey["attributes.json.response"].AsString()
	err = json.Unmarshal([]byte(rawAttrs), &jsonAttrsMap)
	assert.NoError(err)
	assert.Contains(rawAttrs, "128")
}

func TestTestOTelTracer(t *testing.T) {
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
