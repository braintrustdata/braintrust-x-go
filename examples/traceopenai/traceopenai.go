// This example demonstrates OpenAI tracing with Braintrust.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/braintrust/braintrust-x-go/braintrust"
	"github.com/braintrust/braintrust-x-go/braintrust/api"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("recommender")

// Recommender tells you where to get food.
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
	fmt.Println(prompt)

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
	fmt.Println(prompt)

	stream := r.client.Responses.NewStreaming(ctx, responses.ResponseNewParams{
		Model: openai.ChatModelGPT4,
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
	})

	completeText := ""
	for stream.Next() {
		data := stream.Current()
		fmt.Println("\t\tstreaming ... ", data.Delta)
		if data.JSON.Text.IsPresent() {
			completeText = data.Text
		}
	}
	fmt.Println(completeText)

	if err := stream.Err(); err != nil {
		fmt.Println(err)
		return "", err
	}

	return "", nil

}

func main() {
	ctx := context.Background()

	// initialize braintrust tracing with a specific project
	projectName := "traceopenai-example"
	project, err := api.RegisterProject(projectName)
	if err != nil {
		log.Fatal(err)
	}

	opt := braintrust.WithDefaultProjectID(project.ID)

	teardown, err := trace.Quickstart(opt)
	if err != nil {
		log.Fatal(err)
	}
	defer teardown()

	// initialize openai client with tracing middleware
	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	// Make some open ai requests that will be traced.
	recommender := newRecommender(client)
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
