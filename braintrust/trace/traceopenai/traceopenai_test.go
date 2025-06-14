package traceopenai

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
	"github.com/braintrust/braintrust-x-go/braintrust/internal"
	"github.com/braintrust/braintrust-x-go/braintrust/internal/testspan"
)

const TEST_MODEL = "gpt-4o-mini"

// setUpTest is a helper function that sets up a new tracer provider for each test.
// It returns an openai client, an exporter, and a teardown function.
func setUpTest(t *testing.T) (openai.Client, *tracetest.InMemoryExporter, func()) {

	// fail tests if we log warnings.
	internal.FailTestsOnWarnings(t)

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
		diag.ClearLogger()
		err := tp.Shutdown(t.Context())
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

	resp, err := client.Responses.New(t.Context(), responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("hai")},
		Model: TEST_MODEL,
	})
	require.Error(t, err)
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
	require := require.New(t)

	start := time.Now()
	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("What is 13+4?")},
		Model: TEST_MODEL,
	}

	resp, err := client.Responses.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)

	assert.Contains(resp.OutputText(), "17")

	span := flushOne(t, exporter)
	assertSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	_ = ts.Input()
	assert.Contains(ts.AttrString("braintrust.output"), "17")
	_ = ts.Output()

}

func TestOpenAIResponsesKitchenSink(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)
	require := require.New(t)

	prompt := responses.ResponseNewParamsInputUnion{OfString: openai.String("what is 13+4?")}

	// Test with string output
	params := responses.ResponseNewParams{
		Input:             prompt,
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

	start := time.Now()
	resp, err := client.Responses.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check input field
	input := ts.AttrString("braintrust.input")
	assert.Contains(input, "13+4")

	// Check output field
	output := ts.Output()
	text := getResponseText(t, output)
	assert.Contains(text, "17")

	metadata := ts.Metadata()
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
}

func flushSpans(exporter *tracetest.InMemoryExporter) []tracetest.SpanStub {
	// Wait a moment for spans to be exported
	// Get spans without resetting the exporter
	spans := exporter.GetSpans()
	exporter.Reset()
	return spans
}

func flushOne(t *testing.T, exporter *tracetest.InMemoryExporter) tracetest.SpanStub {
	spans := flushSpans(exporter)
	require.Len(t, spans, 1)
	return spans[0]
}

func TestOpenAIResponsesStreamingClose(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	require := require.New(t)
	assert := assert.New(t)

	ctx := t.Context()
	question := "Can you return me a list of the first 15 fibonacci numbers?"

	stream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(question)},
		Model: openai.ChatModelGPT4,
	})

	err := stream.Close()

	require.NoError(err)
	span := flushOne(t, exporter)

	assert.Equal("openai.responses.create", span.Name)
	assert.Equal(codes.Unset, span.Status.Code)
	assert.Equal("", span.Status.Description)
	// FIXME we haven't iterated the body yet, so not much we can assert
}

func TestOpenAIResponsesStreaming(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)
	require := require.New(t)

	ctx := t.Context()
	question := "Can you return me a list of the first 15 fibonacci numbers?"

	start := time.Now()
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
	require.NoError(stream.Err())
	end := time.Now()

	span := flushOne(t, exporter)

	assertSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	output := ts.AttrString("braintrust.output")
	for _, i := range []string{"1", "2", "3", "5", "8", "13"} {
		assert.Contains(completeText, i)
		assert.Contains(output, i)
	}
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
	start := time.Now()
	resp, err := client.Responses.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)

	span := flushOne(t, exporter)

	assertSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	input := ts.AttrString("braintrust.input")
	assert.Contains(input, "3+125")
	assert.Contains(input, "2+2")

	output := ts.Output()
	text := getResponseText(t, output)
	assert.Contains(text, "128")
}

func getResponseText(t *testing.T, resp any) string {
	// 	[]interface {}{map[string]interface {}{"content":[]interface {}{map[string]interface {}{"annotations":[]interface {}{}, "text":"Sure, here is the list of the first 15 Fibonacci numbers:\n\n[0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377]", "type":"output_text"}}, "id":"msg_6815653f1238819281f85b0bd4f95c8e05b761f031bf9c10", "role":"assistant", "status":"completed", "type":"message"}}
	require := require.New(t)

	// Type assert the outer slice
	respSlice, ok := resp.([]interface{})
	require.True(ok, "response should be a slice")
	require.NotEmpty(respSlice, "response slice should not be empty")
	// Get first element and assert it's a map
	firstElem, ok := respSlice[0].(map[string]interface{})
	require.True(ok, "first element should be a map")
	// Get content and assert it's a map
	content, ok := firstElem["content"].([]interface{})
	require.True(ok, "content should be a slice")
	require.NotEmpty(content, "content should not be empty")
	// Get first content element and assert annotations and text
	firstContent, ok := content[0].(map[string]interface{})
	require.True(ok, "first content element should be a map")
	text, ok := firstContent["text"].(string)
	require.True(ok, "text should be a string")
	return text
}

// assertSpanValid asserts all the common properties of a span are valid.
func assertSpanValid(t *testing.T, stub tracetest.SpanStub, start, end time.Time) {
	assert := assert.New(t)

	span := testspan.New(t, stub)
	span.AssertTimingIsValid(start, end)
	span.AssertNameIs("openai.responses.create")
	assert.Equal(codes.Unset, span.Stub.Status.Code)

	metadata := span.Metadata()
	assert.Equal("openai", metadata["provider"])
	assert.Contains(TEST_MODEL, metadata["model"])

	// validate metrics
	metrics := span.Metrics()
	gtz := func(v float64) bool { return v > 0 }
	gtez := func(v float64) bool { return v >= 0 }

	metricToValidator := map[string]func(float64) bool{
		"prompt_tokens":               gtz,
		"completion_tokens":           gtz,
		"tokens":                      gtz,
		"prompt_cached_tokens":        gtez,
		"completion_cached_tokens":    gtez,
		"completion_reasoning_tokens": gtez,
	}

	// this will fail if there are new metrics, but that's ok.
	for n, v := range metrics {
		validator, ok := metricToValidator[n]
		assert.True(ok, "metric %s not found", n)
		assert.True(validator(v), "metric %s is not valid", n)
	}

	// a crude check to make sure all json is parsed
	assert.NotNil(span.Metadata())
	assert.NotNil(span.Input())
	assert.NotNil(span.Output())
}

func TestTestOTelTracer(t *testing.T) {
	_, exporter, teardown := setUpTest(t)
	defer teardown()
	assert := assert.New(t)

	// crudely check we can create and test spans
	spans := flushSpans(exporter)
	assert.Empty(spans)

	tracer := otel.Tracer("test")
	_, span := tracer.Start(t.Context(), "test")
	span.End()

	spans = flushSpans(exporter)
	assert.NotEmpty(spans)
	spans = flushSpans(exporter)
	assert.Empty(spans)
}
