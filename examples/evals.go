package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
)

func registerExperiment(name string, projectID string) (string, error) {
	type ExperimentRequest struct {
		ProjectID string `json:"project_id"`
		Name      string `json:"name"`
		EnsureNew bool   `json:"ensure_new"`
	}

	req := ExperimentRequest{
		ProjectID: projectID,
		Name:      name,
		EnsureNew: true,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", "https://api.braintrust.dev/v1/experiment", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("BRAINTRUST_API_KEY"))

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		ID        string `json:"id"`
		ProjectID string `json:"project_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	return result.ID, nil
}

func main() {
	log.Println("Starting eval")

	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	// create a new braintrust experiemnt by calling
	// api/experiment/register", args)
	projectID := "6df00e6a-704f-44d9-b332-2b3c2690681c"
	experimentID, err := registerExperiment("go-eval-x", projectID)
	if err != nil {
		log.Fatalf("Error registering experiment: %v", err)
	}

	fmt.Println("Experiment ID:", experimentID)

	// set env variables
	os.Setenv("BRAINTRUST_EXPERIMENT_ID", experimentID)

	diag.SetDebugLogger()
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

	eval := eval.Eval[string, string]{
		Cases: []eval.Case[string, string]{
			{Input: "strawberry", Expected: "fruit"},
			{Input: "asparagus", Expected: "vegetable"},
			{Input: "apple", Expected: "fruit"},
			{Input: "banana", Expected: "fruit"},
		},
		Task: getFoodType,
	}
	err = eval.Run()
	if err != nil {
		log.Fatalf("Error running eval: %v", err)
	}

}
