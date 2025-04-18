package main

import (
	"context"
	"fmt"
	"log"

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

	ctx = context.WithValue(ctx, "zipcode", zipcode)

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

func main() {
	ctx := context.Background()
	// tp, err := initTracer()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer tp.Shutdown(ctx)

	client := openai.NewClient(
		option.WithMiddleware(LoggingMiddleware),
	)

	recommender := NewRecommender(client)

	rec, err := recommender.getFoodRec(ctx, "coffee", "11231")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rec)
}
