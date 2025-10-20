// This example demonstrates basic evals with Braintrust.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
)

func main() {

	log.Println("Starting eval")

	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Error starting trace: %v", err)
	}
	defer teardown()

	getFoodType := func(ctx context.Context, food string) (string, error) {
		fmt.Println("getFoodType", food)
		input := fmt.Sprintf("What kind of food is %s?", food)
		params := responses.ResponseNewParams{
			Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(input)},
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String("Return a one word answer."),
		}
		resp, err := client.Responses.New(ctx, params)
		if err != nil {
			return "", err
		}
		return resp.OutputText(), nil
	}

	_, err = eval.Run(context.Background(), eval.Opts[string, string]{
		Project:    "go-sdk-examples",
		Experiment: "go-eval-example",
		Cases: eval.NewCases([]eval.Case[string, string]{
			{Input: "strawberry", Expected: "fruit"},
			{Input: "asparagus", Expected: "vegetable"},
			{Input: "apple", Expected: "fruit"},
			{Input: "banana", Expected: "fruit"},
		}),
		Task: getFoodType,
		Scorers: []eval.Scorer[string, string]{
			eval.NewScorer("fruit_scorer", func(_ context.Context, _, _, result string, _ eval.Metadata) (eval.Scores, error) {
				v := 0.0
				if result == "fruit" {
					v = 1.0
				}
				return eval.S(v), nil
			}),
			eval.NewScorer("vegetable_scorer", func(_ context.Context, _, _, result string, _ eval.Metadata) (eval.Scores, error) {
				v := 0.0
				if result == "vegetable" {
					v = 1.0
				}
				return eval.S(v), nil
			}),
		},
		Tags: []string{"example", "food-classifier", "gpt-4o-mini"},
		Metadata: map[string]interface{}{
			"model":       "gpt-4o-mini",
			"description": "Classifies food items as fruit or vegetable",
		},
		Parallelism: 5,
	})
	if err != nil {
		log.Fatalf("Error running eval: %v", err)
	}
}
