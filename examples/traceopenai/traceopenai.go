package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("openai-examples")

// ChatBot demonstrates using OpenAI chat completions with tracing
type ChatBot struct {
	client openai.Client
}

func NewChatBot(client openai.Client) *ChatBot {
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

// Recommender demonstrates using OpenAI responses API with tracing
type Recommender struct {
	client openai.Client
}

func NewRecommender(client openai.Client) *Recommender {
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
		if data.JSON.Text.IsPresent() {
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

	// Initialize braintrust tracing
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

	bot := NewChatBot(openaiClient)

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

	// ======================
	// RESPONSES API EXAMPLES
	// ======================
	fmt.Println("\n🍕 Responses API Examples")
	fmt.Println("=========================")

	recommender := NewRecommender(openaiClient)

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
