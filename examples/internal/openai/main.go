// OpenAI kitchen sink - tests all major OpenAI features with minimal code. It
// uses v2 of the openai API.

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
)

func main() {
	teardown, err := trace.Quickstart()
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

	// 2. Streaming chat completion
	fmt.Println("2. Streaming...")
	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Count 1 to 3"),
		},
		Model: openai.ChatModelGPT4oMini,
	})
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	fmt.Println("\n✓ Streaming complete")

	// 3. Tool calls
	fmt.Println("3. Tool calls...")
	toolResp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather in SF?"),
		},
		Model: openai.ChatModelGPT4oMini,
		Tools: []openai.ChatCompletionToolParam{
			{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        "get_weather",
					Description: openai.String("Get weather"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{"type": "string"},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(toolResp.Choices) > 0 && len(toolResp.Choices[0].Message.ToolCalls) > 0 {
		fmt.Printf("✓ Tool called: %s\n", toolResp.Choices[0].Message.ToolCalls[0].Function.Name)
	}

	// 4. Responses API
	fmt.Println("4. Responses API...")
	respAPI, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("Recommend pizza in NYC")},
		Model: openai.ChatModelGPT4,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ %s\n", respAPI.OutputText()[:50]+"...")

	// 5. Streaming responses
	fmt.Println("5. Streaming responses...")
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
	fmt.Printf("✓ %s\n", final[:30]+"...")

	// 6. Models.List (untraced endpoint)
	fmt.Println("6. Models.List...")
	models, err := client.Models.List(ctx)
	if err != nil {
		fmt.Printf("✓ Models.List failed: %v\n", err)
	} else {
		fmt.Printf("✓ Models.List succeeded: %d models\n", len(models.Data))
	}

	fmt.Println("\n✅ All OpenAI features tested!")
}
