// This example demonstrates OpenAI tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("openai-examples")

// ChatBot demonstrates using OpenAI chat completions with tracing
type ChatBot struct {
	client openai.Client
}

func newChatBot(client openai.Client) *ChatBot {
	return &ChatBot{
		client: client,
	}
}

func (c *ChatBot) chat(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	ctx, span := tracer.Start(ctx, "chatWithAI")
	defer span.End()

	fmt.Println("--------------------------------")
	fmt.Println("Sending chat request...")

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    openai.ChatModelGPT4oMini,
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("no response content")
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *ChatBot) streamChat(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) error {
	ctx, span := tracer.Start(ctx, "streamChatWithAI")
	defer span.End()

	fmt.Println("--------------------------------")
	fmt.Println("Streaming chat response...")

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    openai.ChatModelGPT4oMini,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	fmt.Println()

	return stream.Err()
}

func (c *ChatBot) toolCallsChat(ctx context.Context, prompt string) error {
	ctx, span := tracer.Start(ctx, "toolCallsChat")
	defer span.End()

	fmt.Println("--------------------------------")
	fmt.Println("Streaming tool calls chat...")

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant with access to tools."),
			openai.UserMessage(prompt),
		},
		Model: openai.ChatModelGPT4oMini,
		Tools: []openai.ChatCompletionToolParam{
			{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        "get_current_weather",
					Description: openai.String("Get the current weather in a given location"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
							"unit": map[string]interface{}{
								"type": "string",
								"enum": []string{"celsius", "fahrenheit"},
							},
						},
						"required": []string{"location"},
					},
				},
			},
			{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        "get_current_time",
					Description: openai.String("Get the current time in a given timezone"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"timezone": map[string]interface{}{
								"type":        "string",
								"description": "The timezone, e.g. America/New_York",
							},
						},
						"required": []string{"timezone"},
					},
				},
			},
		},
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	var toolCalls []map[string]interface{}
	var content string

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			// Handle regular content
			if choice.Delta.Content != "" {
				content += choice.Delta.Content
				fmt.Print(choice.Delta.Content)
			}

			// Handle tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					if tc.ID != "" {
						// New tool call
						toolCall := map[string]interface{}{
							"id":   tc.ID,
							"type": tc.Type,
							"function": map[string]interface{}{
								"name":      tc.Function.Name,
								"arguments": tc.Function.Arguments,
							},
						}
						toolCalls = append(toolCalls, toolCall)
						fmt.Printf("\n🔧 Tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments)
					} else if len(toolCalls) > 0 {
						// Continue existing tool call
						lastCall := toolCalls[len(toolCalls)-1]
						if function, ok := lastCall["function"].(map[string]interface{}); ok {
							if args, ok := function["arguments"].(string); ok {
								function["arguments"] = args + tc.Function.Arguments
								fmt.Print(tc.Function.Arguments)
							}
						}
					}
				}
			}

			if choice.FinishReason == "tool_calls" {
				fmt.Println("\n🎯 Model wants to call tools!")
				// Simulate tool call execution
				for _, toolCall := range toolCalls {
					if function, ok := toolCall["function"].(map[string]interface{}); ok {
						if name, ok := function["name"].(string); ok {
							var result string
							switch name {
							case "get_current_weather":
								result = `{"location":"San Francisco, CA","temperature":"72°F","condition":"sunny"}`
							case "get_current_time":
								result = `{"timezone":"America/New_York","time":"2024-01-15 14:30:00 EST"}`
							default:
								result = `{"error":"Unknown function"}`
							}
							fmt.Printf("📡 Tool result: %s\n", result)
						}
					}
				}
			}
		}
	}
	fmt.Println()

	return stream.Err()
}

// Recommender demonstrates using OpenAI responses API with tracing
type Recommender struct {
	client openai.Client
}

func newRecommender(client openai.Client) *Recommender {
	return &Recommender{
		client: client,
	}
}

func (r *Recommender) getFoodRec(ctx context.Context, food string, zipcode string) (string, error) {
	ctx, span := tracer.Start(ctx, "getFoodRec")
	defer span.End()

	prompt := fmt.Sprintf("Recommend a place to get %s in zipcode %s.", food, zipcode)

	fmt.Println("--------------------------------")
	fmt.Println("Getting food recommendation:", prompt)

	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
		Model: openai.ChatModelGPT4,
	}

	resp, err := r.client.Responses.New(ctx, params)
	if err != nil {
		return "", err
	}

	return resp.OutputText(), nil
}

func (r *Recommender) getDrinkRec(ctx context.Context, drink, vibe, zipcode string) (string, error) {
	ctx, span := tracer.Start(ctx, "getDrinkRec")
	defer span.End()

	prompt := fmt.Sprintf("Recommend a place to get %s with vibe %s in zipcode %s.", drink, vibe, zipcode)
	fmt.Println("--------------------------------")
	fmt.Println("Streaming drink recommendation:", prompt)

	stream := r.client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: openai.ChatModelGPT4,
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
	})

	completeText := ""
	for stream.Next() {
		data := stream.Current()
		fmt.Print(".") // Progress indicator
		if data.Text != "" {
			completeText = data.Text
		}
	}
	fmt.Println()

	if err := stream.Err(); err != nil {
		return "", err
	}

	return completeText, nil
}

func main() {
	fmt.Println("🧠 Braintrust OpenAI Tracing Examples")
	fmt.Println("=====================================")

	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// Initialize OpenAI client with tracing middleware
	openaiClient := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	ctx := context.Background()

	ctx, rootSpan := tracer.Start(ctx, "openai-examples")
	defer rootSpan.End()

	// ======================
	// CHAT COMPLETIONS EXAMPLES
	// ======================
	fmt.Println("\n🗨️  Chat Completions Examples")
	fmt.Println("=============================")

	bot := newChatBot(openaiClient)

	// Example 1: Simple chat completion
	fmt.Println("\n1. Simple Chat Completion")
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("What is the capital of France?"),
	}

	response, err := bot.chat(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", response)

	// Example 2: Streaming chat completion
	fmt.Println("\n2. Streaming Chat Completion")
	streamMessages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Count from 1 to 5 slowly."),
	}

	if err := bot.streamChat(ctx, streamMessages); err != nil {
		log.Fatal(err)
	}

	// Example 3: Multi-turn conversation
	fmt.Println("\n3. Multi-turn Conversation")
	conversation := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a math tutor."),
		openai.UserMessage("What is 15 + 27?"),
	}

	response, err = bot.chat(ctx, conversation)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Math response: %s\n", response)

	// Add assistant response to conversation
	conversation = append(conversation, openai.AssistantMessage(response))
	conversation = append(conversation, openai.UserMessage("Now divide that by 3"))

	response, err = bot.chat(ctx, conversation)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Follow-up response: %s\n", response)

	// Example 6: Tool calls with streaming
	fmt.Println("\n6. Streaming Tool Calls")
	if err := bot.toolCallsChat(ctx, "What's the weather like in San Francisco and what time is it in New York?"); err != nil {
		log.Fatal(err)
	}

	// ======================
	// RESPONSES API EXAMPLES
	// ======================
	fmt.Println("\n🍕 Responses API Examples")
	fmt.Println("=========================")

	recommender := newRecommender(openaiClient)

	// Example 4: Simple responses request
	fmt.Println("\n4. Simple Food Recommendation")
	foodRec, err := recommender.getFoodRec(ctx, "pizza", "11231")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Food recommendation: %s\n", foodRec)

	// Example 5: Streaming responses request
	fmt.Println("\n5. Streaming Drink Recommendation")
	drinkRec, err := recommender.getDrinkRec(ctx, "beer", "chill", "11231")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Drink recommendation: %s\n", drinkRec)

	fmt.Println("\n✅ All examples completed successfully!")
	fmt.Println("Check your Braintrust dashboard to view the traces.")
}
