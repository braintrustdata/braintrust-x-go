package main

import (
	"context"
	"fmt"
	"log"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
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

	eval1, err := eval.NewWithOpts(
		eval.Options{
			ProjectName:    "go-eval-project",
			ExperimentName: "go-eval-x",
		},
		[]eval.Case[string, string]{
			{Input: "strawberry", Expected: "fruit"},
			{Input: "asparagus", Expected: "vegetable"},
			{Input: "apple", Expected: "fruit"},
			{Input: "banana", Expected: "fruit"},
		},
		getFoodType,
		[]eval.Scorer[string, string]{
			eval.NewScorer("fruit_scorer", func(ctx context.Context, input, expected, result string) (float64, error) {
				if result == "fruit" {
					return 1.0, nil
				}
				return 0.0, nil
			}),
			eval.NewScorer("vegetable_scorer", func(ctx context.Context, input, expected, result string) (float64, error) {
				if result == "vegetable" {
					return 1.0, nil
				}
				return 0.0, nil
			}),
		},
	)
	if err != nil {
		log.Fatalf("Error creating eval: %v", err)
	}
	err = eval1.Run()
	if err != nil {
		log.Fatalf("Error running eval: %v", err)
	}
}
