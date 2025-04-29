package traceopenai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
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

	SetLogger(&failTestLogger{t: t})

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
		SetLogger(noopLogger{})
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
	assert.Equal("openai.responses.create", span.Name)
	assert.Equal(codes.Unset, span.Status.Code)
	assert.Equal("", span.Status.Description)

	valsByKey := toValuesByKey(span.Attributes)

	// Check metadata fields
	var metadata map[string]any
	require.NotNil(t, valsByKey["braintrust.metadata"])
	err = json.Unmarshal([]byte(valsByKey["braintrust.metadata"].AsString()), &metadata)
	assert.NoError(err)
	assert.Equal("openai", metadata["provider"])
	assert.Contains(metadata["model"], TEST_MODEL)

	// Check metrics fields
	var metrics map[string]int64
	err = json.Unmarshal([]byte(valsByKey["braintrust.metrics"].AsString()), &metrics)
	assert.NoError(err)
	assert.Greater(metrics["input_tokens"], int64(0))
	assert.Greater(metrics["output_tokens"], int64(0))
	assert.Greater(metrics["total_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["input_tokens_details.cached_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["output_tokens_details.reasoning_tokens"], int64(0))

	// Check input field
	var input any
	err = json.Unmarshal([]byte(valsByKey["braintrust.input"].AsString()), &input)
	assert.NoError(err)
	assert.Contains(valsByKey["braintrust.input"].AsString(), "13+4")

	// Check output field
	var output any
	err = json.Unmarshal([]byte(valsByKey["braintrust.output"].AsString()), &output)
	assert.NoError(err)
	assert.Contains(valsByKey["braintrust.output"].AsString(), "17")
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

	// Output field
	outputText := resp.OutputText()
	assert.Contains(outputText, "17")

	metadata := make(map[string]any)
	rawAttrs := valsByKey["braintrust.metadata"].AsString()
	err = json.Unmarshal([]byte(rawAttrs), &metadata)
	assert.NoError(err)
	assert.Equal("openai", metadata["provider"])
	assert.Equal(TEST_MODEL, metadata["model"])
	assert.Equal("Answer the question in a concise manner.", metadata["instructions"])
	assert.Equal("test user", metadata["user"])
	assert.Equal(0.5, metadata["temperature"])
	assert.Equal(1.0, metadata["top_p"])
	assert.Equal(true, metadata["parallel_tool_calls"])
	assert.Equal(false, metadata["store"])
	assert.Equal("auto", metadata["truncation"])
	assert.Equal(100.0, metadata["max_output_tokens"])

	// Check JSON serialized fields
	var inputStr any
	rawInput := valsByKey["braintrust.input"].AsString()
	err = json.Unmarshal([]byte(rawInput), &inputStr)
	assert.NoError(err)
	assert.Contains(inputStr, "13+4")

	var output any
	rawOutput := valsByKey["braintrust.output"].AsString()
	err = json.Unmarshal([]byte(rawOutput), &output)
	assert.NoError(err)
	assert.NotNil(output)
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

func TestOpenAIResponsesStreamingClose(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert, require := assert.New(t), require.New(t)

	ctx := context.Background()
	question := "Can you return me a list of the first 15 fibonacci numbers?"

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(question)},
		Model: openai.ChatModelGPT4,
	})

	err := stream.Close()
	require.NoError(err)

	spans := flushSpans(exporter)
	require.Len(spans, 1)
	span := spans[0]
	assert.Equal(span.Name, "openai.responses.create")
	assert.Equal(codes.Unset, span.Status.Code)
	assert.Equal("", span.Status.Description)
}

func TestOpenAIResponsesStreaming(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert, require := assert.New(t), require.New(t)

	ctx := context.Background()
	question := "Can you return me a list of the first 15 fibonacci numbers?"

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(question)},
		Model: openai.ChatModelGPT4,
	})

	var completeText string

	for stream.Next() {
		data := stream.Current()
		if data.JSON.Text.IsPresent() {
			completeText = data.Text
		}
	}

	spans := flushSpans(exporter)
	require.Len(spans, 1)
	span := spans[0]
	assert.Equal(span.Name, "openai.responses.create")
	assert.Equal(codes.Unset, span.Status.Code)
	assert.Equal("", span.Status.Description)

	valsByKey := toValuesByKey(span.Attributes)

	metadata := make(map[string]any)
	rawMetadata := valsByKey["braintrust.metadata"].AsString()
	err := json.Unmarshal([]byte(rawMetadata), &metadata)
	assert.NoError(err)
	assert.Equal("openai", metadata["provider"])

	var output any
	rawOutput := valsByKey["braintrust.output"].AsString()
	err = json.Unmarshal([]byte(rawOutput), &output)
	assert.NoError(err)
	for _, i := range []string{"1", "2", "3", "5", "8", "13"} {
		assert.Contains(completeText, i)
		assert.Contains(rawOutput, i)
	}
	rawMetrics := valsByKey["braintrust.metrics"].AsString()
	var metrics map[string]int64
	err = json.Unmarshal([]byte(rawMetrics), &metrics)
	assert.NoError(err)
	assert.Greater(metrics["input_tokens"], int64(0))
	assert.Greater(metrics["output_tokens"], int64(0))
	assert.Greater(metrics["total_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["input_tokens_details.cached_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["output_tokens_details.reasoning_tokens"], int64(0))
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

	rawMetadata := valsByKey["braintrust.metadata"].AsString()
	var metadata map[string]any
	err = json.Unmarshal([]byte(rawMetadata), &metadata)
	assert.NoError(err)
	assert.Equal("openai", metadata["provider"])
	assert.Equal(TEST_MODEL, metadata["model"])

	rawMetrics := valsByKey["braintrust.metrics"].AsString()
	var metrics map[string]int64
	err = json.Unmarshal([]byte(rawMetrics), &metrics)
	assert.NoError(err)

	assert.Greater(metrics["input_tokens"], int64(0))
	assert.Greater(metrics["output_tokens"], int64(0))
	assert.Greater(metrics["total_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["input_tokens_details.cached_tokens"], int64(0))
	assert.GreaterOrEqual(metrics["output_tokens_details.reasoning_tokens"], int64(0))

	var jsonInput any
	input := valsByKey["braintrust.input"].AsString()
	err = json.Unmarshal([]byte(input), &jsonInput)
	assert.NoError(err)
	assert.Contains(input, "3+125")

	var outputMap any
	output := valsByKey["braintrust.output"].AsString()
	err = json.Unmarshal([]byte(output), &outputMap)
	assert.NoError(err)
	//assert.Contains(outputMap["text"].(string), "128")
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

type failTestLogger struct {
	t *testing.T
}

func (l *failTestLogger) Debugf(format string, args ...any) {}

func (l *failTestLogger) Warnf(format string, args ...any) {
	l.t.Fatalf("%s\n%s", fmt.Sprintf(format, args...), string(debug.Stack()))
}
