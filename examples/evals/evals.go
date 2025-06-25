// This example demonstrates basic evaluation functionality with Braintrust.
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

	experimentID, err := eval.ResolveProjectExperimentID("go-eval-x", "go-eval-project")
	if err != nil {
		log.Fatalf("Failed to resolve experiment: %v", err)
	}

	eval1 := eval.New(experimentID,
		eval.NewCases([]eval.Case[string, string]{
			{Input: "strawberry", Expected: "fruit"},
			{Input: "asparagus", Expected: "vegetable"},
			{Input: "apple", Expected: "fruit"},
			{Input: "banana", Expected: "fruit"},
		}),
		getFoodType,
		[]eval.Scorer[string, string]{
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
	)
	err = eval1.Run(context.Background())
	if err != nil {
		log.Fatalf("Error running eval: %v", err)
	}
}
