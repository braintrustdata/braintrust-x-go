package main

import (
	"context"
	"fmt"
	"log"

	"github.com/braintrust/braintrust-x-go/traceopenai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("recommender")

// Recommender tells you where to get food.
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

	prompt := fmt.Sprintf("Recommend a place to get %s with vibe %sin zipcode %s.", drink, vibe, zipcode)

	stream := r.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Seed: openai.Int(0),
	})

	for stream.Next() {
		stream.Current()
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	return "", nil

}

func main() {
	ctx := context.Background()

	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := tp.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()

	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	recommender := NewRecommender(client)

	ctx, span := tracer.Start(ctx, "recommendations")
	defer span.End()

	rec, err := recommender.getFoodRec(ctx, "pizza", "11231")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rec)

	rec, err = recommender.getDrinkRec(ctx, "beer", "chill", "11231")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rec)

}
