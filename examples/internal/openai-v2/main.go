// OpenAI kitchen sink - tests all major OpenAI features with minimal code. It
// uses v2 of the openai API.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/conversations"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
)

func main() {
	teardown, err := trace.Quickstart(braintrust.WithDefaultProject("go-sdk-internal-examples"))
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	client := openai.NewClient(option.WithMiddleware(traceopenai.Middleware))
	ctx := context.Background()

	// 1. Chat completion
	fmt.Println("1. Chat completion...")
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Say hello"),
		},
		Model: openai.ChatModelGPT4oMini,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ %s\n", resp.Choices[0].Message.Content)

	// 2. Multiple messages (conversation history)
	fmt.Println("2. Multiple messages...")
	multiResp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant."),
			openai.UserMessage("What is the capital of France?"),
			openai.AssistantMessage("The capital of France is Paris."),
			openai.UserMessage("What is its population?"),
		},
		Model: openai.ChatModelGPT4oMini,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ %s\n", multiResp.Choices[0].Message.Content)

	// 3. Streaming chat completion
	fmt.Println("3. Streaming...")
	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Count 1 to 3"),
		},
		Model: openai.ChatModelGPT4oMini,
	})
	fmt.Print("✓ ")
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println()

	// 4. Chat with tools
	fmt.Println("4. Chat with tools...")
	toolResp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather in San Francisco?"),
		},
		Model: openai.ChatModelGPT4oMini,
		Tools: []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
				Name:        "get_weather",
				Description: openai.String("Get the current weather in a location"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
					},
					"required": []string{"location"},
				},
			}),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(toolResp.Choices) > 0 && len(toolResp.Choices[0].Message.ToolCalls) > 0 {
		toolCall := toolResp.Choices[0].Message.ToolCalls[0]
		fmt.Printf("✓ Tool call: %s(%s)\n", toolCall.Function.Name, toolCall.Function.Arguments)
	} else {
		fmt.Printf("✓ Response: %s\n", toolResp.Choices[0].Message.Content)
	}

	// 5. Streaming with tools
	fmt.Println("5. Streaming with tools...")
	toolStream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather in Tokyo?"),
		},
		Model: openai.ChatModelGPT4oMini,
		Tools: []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
				Name:        "get_weather",
				Description: openai.String("Get the current weather in a location"),
				Parameters: shared.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state or country",
						},
					},
					"required": []string{"location"},
				},
			}),
		},
	})
	var toolCallName string
	var toolCallArgs string
	for toolStream.Next() {
		chunk := toolStream.Current()
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			tc := chunk.Choices[0].Delta.ToolCalls[0]
			if tc.Function.Name != "" {
				toolCallName = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCallArgs += tc.Function.Arguments
			}
		}
	}
	if err := toolStream.Err(); err != nil {
		log.Fatal(err)
	}
	if toolCallName != "" {
		fmt.Printf("✓ Streamed tool call: %s(%s)\n", toolCallName, toolCallArgs)
	}

	// 6. Responses API
	fmt.Println("6. Responses API...")
	respAPI, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("Recommend pizza in NYC")},
		Model: openai.ChatModelGPT4,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ %s\n", respAPI.OutputText()[:50]+"...")

	// 7. Streaming responses
	fmt.Println("7. Streaming responses...")
	respStream := client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: openai.ChatModelGPT4,
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("Quick coffee rec")},
	})
	var final string
	for respStream.Next() {
		data := respStream.Current()
		if data.Text != "" {
			final = data.Text
		}
	}
	if err := respStream.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ %s\n", final[:30]+"...")

	// 8. Conversations API (new in v2)
	fmt.Println("8. Conversations API...")
	conv, err := client.Conversations.New(ctx, conversations.ConversationNewParams{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Conversation created: %s\n", conv.ID)

	// 9. Models.List (untraced endpoint)
	fmt.Println("9. Models.List...")
	models, err := client.Models.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Models.List succeeded: %d models\n", len(models.Data))

	fmt.Println("\n✅ All OpenAI features tested!")
}
