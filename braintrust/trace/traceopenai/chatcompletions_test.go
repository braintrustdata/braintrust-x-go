package traceopenai

import (
	"fmt"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/braintrust/braintrust-x-go/braintrust/internal/testspan"
)

func TestOpenAIChatCompletions(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	t.Cleanup(teardown)
	assert := assert.New(t)
	require := require.New(t)

	// Test basic chat completion
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What is 2+2?"),
		},
		Model: testModel,
	}

	start := time.Now()
	resp, err := client.Chat.Completions.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)
	fmt.Println(resp)

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertChatSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check that the span name is correct
	assert.Equal("openai.chat.completions.create", span.Name)

	// Check input field
	input := ts.AttrString("braintrust.input")
	assert.Contains(input, "What is 2+2?")

	// Check output field
	output := ts.Output()
	assert.NotNil(output)
	// The output should contain choices array
	choices, ok := output.([]interface{})
	require.True(ok, "output should be an array of choices")
	require.Greater(len(choices), 0, "should have at least one choice")

	// Check first choice has message with content
	firstChoice, ok := choices[0].(map[string]interface{})
	require.True(ok, "first choice should be a map")
	message, ok := firstChoice["message"].(map[string]interface{})
	require.True(ok, "choice should have a message")
	content, ok := message["content"].(string)
	require.True(ok, "message should have content string")
	assert.Contains(content, "4")

	// Check metadata
	metadata := ts.Metadata()
	assert.Equal("openai", metadata["provider"])
	assert.Equal("/v1/chat/completions", metadata["endpoint"])
	assert.Equal(testModel, metadata["model"])

	// Check metrics
	metrics := ts.Metrics()
	assert.Greater(metrics["prompt_tokens"], float64(0))
	assert.Greater(metrics["completion_tokens"], float64(0))
	assert.Greater(metrics["tokens"], float64(0))
}

func TestOpenAIChatCompletionsStreaming(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	t.Cleanup(teardown)
	assert := assert.New(t)
	require := require.New(t)

	// Test streaming chat completion
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Count from 1 to 3"),
		},
		Model: testModel,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}

	start := time.Now()
	stream := client.Chat.Completions.NewStreaming(t.Context(), params)

	var fullContent string
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fullContent += chunk.Choices[0].Delta.Content
		}
	}
	end := time.Now()

	require.NoError(stream.Err())
	require.NotEmpty(fullContent)

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertChatSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check that the span name is correct
	assert.Equal("openai.chat.completions.create", span.Name)

	// Check input field
	input := ts.AttrString("braintrust.input")
	assert.Contains(input, "Count from 1 to 3")

	// Check output field
	output := ts.Output()
	assert.NotNil(output)

	// Check metadata indicates streaming
	metadata := ts.Metadata()
	assert.Equal(true, metadata["stream"])
}

func TestOpenAIChatCompletionsWithTools(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	t.Cleanup(teardown)
	assert := assert.New(t)
	require := require.New(t)

	// Test chat completion with function calling
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather in San Francisco?"),
		},
		Model: testModel,
		Tools: []openai.ChatCompletionToolParam{
			{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        "get_weather",
					Description: openai.String("Get the weather for a location"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	}

	start := time.Now()
	resp, err := client.Chat.Completions.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertChatSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check that tools are in metadata
	metadata := ts.Metadata()
	tools, ok := metadata["tools"].([]interface{})
	require.True(ok, "metadata should contain tools array")
	assert.Len(tools, 1)
}

func TestOpenAIChatCompletionsWithSystemMessage(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	t.Cleanup(teardown)
	assert := assert.New(t)
	require := require.New(t)

	// Test with system message
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant that speaks like a pirate."),
			openai.UserMessage("Hello, how are you?"),
		},
		Model:       testModel,
		Temperature: openai.Float(0.7),
		MaxTokens:   openai.Int(100),
	}

	start := time.Now()
	resp, err := client.Chat.Completions.New(t.Context(), params)
	end := time.Now()
	require.NoError(err)
	require.NotNil(resp)

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertChatSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check input contains both messages
	input := ts.AttrString("braintrust.input")
	assert.Contains(input, "pirate")
	assert.Contains(input, "Hello, how are you?")

	// Check metadata contains temperature and max_tokens
	metadata := ts.Metadata()
	assert.Equal(0.7, metadata["temperature"])
	assert.Equal(100.0, metadata["max_tokens"]) // JSON numbers are floats
}

func TestOpenAIChatCompletionsStreamingToolCalls(t *testing.T) {
	client, exporter, teardown := setUpTest(t)
	t.Cleanup(teardown)
	assert := assert.New(t)
	require := require.New(t)

	// Test streaming chat completion with tool calls
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather in San Francisco?"),
		},
		Model: testModel,
		Tools: []openai.ChatCompletionToolParam{
			{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        "get_weather",
					Description: openai.String("Get weather for a location"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}

	start := time.Now()
	stream := client.Chat.Completions.NewStreaming(t.Context(), params)

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			// We don't need to process the chunks here, just consume them
			_ = choice.Delta.Content
			_ = choice.Delta.ToolCalls
		}
	}
	end := time.Now()

	require.NoError(stream.Err())

	// Wait for spans to be exported
	span := flushOne(t, exporter)

	assertChatSpanValid(t, span, start, end)

	ts := testspan.New(t, span)

	// Check that the span name is correct
	assert.Equal("openai.chat.completions.create", span.Name)

	// Check output field - should be properly structured
	output := ts.Output()
	assert.NotNil(output)

	// The output should be an array with one choice
	choices, ok := output.([]interface{})
	require.True(ok, "output should be an array of choices")
	require.Len(choices, 1, "should have exactly one choice")

	firstChoice, ok := choices[0].(map[string]interface{})
	require.True(ok, "first choice should be a map")

	// Verify choice structure
	assert.Equal(0.0, firstChoice["index"]) // JSON numbers are floats
	assert.Nil(firstChoice["logprobs"])

	// Check message structure
	message, ok := firstChoice["message"].(map[string]interface{})
	require.True(ok, "choice should have a message")

	// Role should be set (not nil/null)
	role := message["role"]
	assert.NotNil(role, "role should be set for tool calls")
	if role != nil {
		assert.Equal("assistant", role)
	}

	// Check metadata contains tools
	metadata := ts.Metadata()
	tools, ok := metadata["tools"].([]interface{})
	require.True(ok, "metadata should contain tools array")
	assert.Len(tools, 1)
}

func TestStreamingToolCallsPostprocessing(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ct := newChatCompletionsTracer()

	t.Run("EmptyResults", func(t *testing.T) {
		result := ct.postprocessStreamingResults([]map[string]any{})
		require.Len(result, 1)
		choice := result[0]

		message, ok := choice["message"].(map[string]interface{})
		require.True(ok)
		assert.Nil(message["role"])
		assert.Equal("", message["content"])
		assert.Nil(message["tool_calls"])
	})

	t.Run("SingleToolCall", func(t *testing.T) {
		// Simulate a streaming response with a single tool call
		results := []map[string]any{
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"role": "assistant",
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []interface{}{
								map[string]any{
									"id":   "call_123",
									"type": "function",
									"function": map[string]any{
										"name":      "get_weather",
										"arguments": `{"location":"`,
									},
								},
							},
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []interface{}{
								map[string]any{
									"function": map[string]any{
										"arguments": `San Francisco"}`,
									},
								},
							},
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index":         0,
						"finish_reason": "tool_calls",
						"delta":         map[string]any{},
					},
				},
			},
		}

		result := ct.postprocessStreamingResults(results)
		require.Len(result, 1)

		choice := result[0]
		message, ok := choice["message"].(map[string]interface{})
		require.True(ok)

		assert.Equal("assistant", message["role"])
		assert.Equal("", message["content"])
		assert.Equal("tool_calls", choice["finish_reason"])

		toolCalls, ok := message["tool_calls"].([]interface{})
		require.True(ok, "should have tool_calls")
		require.Len(toolCalls, 1)

		toolCall, ok := toolCalls[0].(map[string]interface{})
		require.True(ok)
		assert.Equal("call_123", toolCall["id"])
		assert.Equal("function", toolCall["type"])

		function, ok := toolCall["function"].(map[string]interface{})
		require.True(ok)
		assert.Equal("get_weather", function["name"])
		assert.Equal(`{"location":"San Francisco"}`, function["arguments"])
	})

	t.Run("MultipleToolCalls", func(t *testing.T) {
		// Simulate a streaming response with multiple tool calls
		results := []map[string]any{
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "get_weather",
										"arguments": `{"location":"SF"}`,
									},
								},
							},
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []interface{}{
								map[string]any{
									"id":   "call_2",
									"type": "function",
									"function": map[string]any{
										"name":      "get_time",
										"arguments": `{"timezone":"EST"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		result := ct.postprocessStreamingResults(results)
		require.Len(result, 1)

		choice := result[0]
		message, ok := choice["message"].(map[string]interface{})
		require.True(ok)

		toolCalls, ok := message["tool_calls"].([]interface{})
		require.True(ok, "should have tool_calls")
		require.Len(toolCalls, 2)

		// Check first tool call
		toolCall1, ok := toolCalls[0].(map[string]interface{})
		require.True(ok)
		assert.Equal("call_1", toolCall1["id"])
		function1, ok := toolCall1["function"].(map[string]interface{})
		require.True(ok)
		assert.Equal("get_weather", function1["name"])

		// Check second tool call
		toolCall2, ok := toolCalls[1].(map[string]interface{})
		require.True(ok)
		assert.Equal("call_2", toolCall2["id"])
		function2, ok := toolCall2["function"].(map[string]interface{})
		require.True(ok)
		assert.Equal("get_time", function2["name"])
	})

	t.Run("ContentAndToolCalls", func(t *testing.T) {
		// Test mixed content and tool calls
		results := []map[string]any{
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"role":    "assistant",
							"content": "I'll help you with that. ",
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"content": "Let me check the weather.",
						},
					},
				},
			},
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"tool_calls": []interface{}{
								map[string]any{
									"id":   "call_weather",
									"type": "function",
									"function": map[string]any{
										"name":      "get_weather",
										"arguments": `{"location":"NYC"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		result := ct.postprocessStreamingResults(results)
		require.Len(result, 1)

		choice := result[0]
		message, ok := choice["message"].(map[string]interface{})
		require.True(ok)

		assert.Equal("assistant", message["role"])
		assert.Equal("I'll help you with that. Let me check the weather.", message["content"])

		toolCalls, ok := message["tool_calls"].([]interface{})
		require.True(ok)
		require.Len(toolCalls, 1)
	})

	t.Run("EmptyToolCallsArray", func(t *testing.T) {
		// Test the edge case that caused the original bug
		results := []map[string]any{
			{
				"choices": []interface{}{
					map[string]any{
						"index": 0,
						"delta": map[string]any{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]any{
									"id":   "call_first",
									"type": "function",
									"function": map[string]any{
										"name":      "test_function",
										"arguments": `{"param":"value"}`,
									},
								},
							},
						},
					},
				},
			},
		}

		// This should not panic (the bug we fixed)
		result := ct.postprocessStreamingResults(results)
		require.Len(result, 1)

		choice := result[0]
		message, ok := choice["message"].(map[string]interface{})
		require.True(ok)
		toolCalls, ok := message["tool_calls"].([]interface{})
		require.True(ok)
		require.Len(toolCalls, 1)
	})
}

// assertChatSpanValid asserts all the common properties of a chat completion span are valid.
func assertChatSpanValid(t *testing.T, stub tracetest.SpanStub, start, end time.Time) {
	t.Helper()
	assert := assert.New(t)

	span := testspan.New(t, stub)
	span.AssertTimingIsValid(start, end)
	span.AssertNameIs("openai.chat.completions.create")
	assert.Equal(codes.Unset, span.Stub.Status.Code)

	metadata := span.Metadata()
	assert.Equal("openai", metadata["provider"])
	assert.Equal("/v1/chat/completions", metadata["endpoint"])
	assert.Contains(testModel, metadata["model"])

	// validate metrics
	metrics := span.Metrics()
	gtz := func(v float64) bool { return v > 0 }
	gtez := func(v float64) bool { return v >= 0 }

	metricToValidator := map[string]func(float64) bool{
		"prompt_tokens":                         gtz,
		"completion_tokens":                     gtz,
		"tokens":                                gtz,
		"prompt_cached_tokens":                  gtez,
		"completion_cached_tokens":              gtez,
		"completion_reasoning_tokens":           gtez,
		"completion_accepted_prediction_tokens": gtez,
		"completion_rejected_prediction_tokens": gtez,
		"completion_audio_tokens":               gtez,
		"prompt_audio_tokens":                   gtez,
	}

	// Validate known metrics, but allow unknown metrics to pass through
	for n, v := range metrics {
		validator, ok := metricToValidator[n]
		if !ok {
			// Unknown metric - just log it but don't fail the test
			t.Logf("Unknown metric %s with value %v - this is likely a new OpenAI metric", n, v)
			continue
		}
		assert.True(validator(v), "metric %s is not valid", n)
	}

	// a crude check to make sure all json is parsed
	assert.NotNil(span.Metadata())
	assert.NotNil(span.Input())
	assert.NotNil(span.Output())
}
