// Package main demonstrates a simple dataset evaluation example for Braintrust.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/braintrust/braintrust-x-go/braintrust/api"
	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
)

// datasetWrapper implements eval.Dataset by wrapping api.Dataset
type datasetWrapper struct {
	dataset *api.Dataset
}

// Next converts api.DatasetEvent to eval.Case
func (w *datasetWrapper) Next() (eval.Case[string, string], error) {
	event, err := w.dataset.Next()
	if err != nil {
		if err.Error() != "EOF" {
			fmt.Printf("📤 Dataset fetch ended: %v\n", err)
		}
		return eval.Case[string, string]{}, err
	}

	var input, expected string

	// Convert input to string
	if event.Input != nil {
		if inputBytes, err := json.Marshal(event.Input); err == nil {
			if err := json.Unmarshal(inputBytes, &input); err != nil {
				log.Printf("Failed to unmarshal input: %v", err)
			}
		}
	}

	// Convert expected to string
	if event.Expected != nil {
		if expectedBytes, err := json.Marshal(event.Expected); err == nil {
			if err := json.Unmarshal(expectedBytes, &expected); err != nil {
				log.Printf("Failed to unmarshal expected: %v", err)
			}
		}
	}

	// fmt.Printf("📊 Fetched case: input='%s', expected='%s'\n", input, expected)
	return eval.Case[string, string]{
		Input:    input,
		Expected: expected,
	}, nil
}

func main() {
	// Initialize OpenTelemetry tracing for Braintrust
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer teardown() // Ensure traces are flushed on exit

	// First, create a project
	project, err := api.RegisterProject("Dataset Example Project")
	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}
	fmt.Printf("📁 Created project: %s (ID: %s)\n", project.Name, project.ID)

	// Create a dataset with timestamp to make it unique
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	datasetInfo, err := api.CreateDataset(api.DatasetRequest{
		ProjectID:   project.ID,
		Name:        "Sample Text Dataset " + timestamp,
		Description: "A sample dataset for demonstrating dataset evaluation",
	})
	if err != nil {
		log.Fatalf("Failed to create dataset: %v", err)
	}
	fmt.Printf("📊 Created dataset: %s (ID: %s)\n", datasetInfo.Name, datasetInfo.ID)

	// Insert some sample data into the dataset
	sampleEvents := []api.DatasetEvent{
		{Input: "Hello", Expected: "Processed: Hello"},
		{Input: "World", Expected: "Processed: World"},
		{Input: "Braintrust", Expected: "Processed: Braintrust"},
		{Input: "Dataset", Expected: "Processed: Dataset"},
	}

	err = api.InsertDatasetEvents(datasetInfo.ID, sampleEvents)
	if err != nil {
		log.Fatalf("Failed to insert events: %v", err)
	}
	fmt.Printf("📝 Inserted %d events into dataset\n", len(sampleEvents))

	// Verify the data was inserted by fetching it back
	fmt.Println("\n🔍 Querying dataset to verify data...")
	fetchResp, err := api.FetchDatasetEvents(datasetInfo.ID, api.DatasetFetchRequest{Limit: 10})
	if err != nil {
		log.Printf("Warning: Failed to fetch events for verification: %v", err)
	} else {
		fmt.Printf("📊 Found %d events in dataset:\n", len(fetchResp.Events))
		for i, rawEvent := range fetchResp.Events {
			var event api.DatasetEvent
			if err := json.Unmarshal(rawEvent, &event); err == nil {
				fmt.Printf("  %d. Input: %v, Expected: %v\n", i+1, event.Input, event.Expected)
			} else {
				fmt.Printf("  %d. Failed to unmarshal event: %v\n", i+1, err)
			}
		}
		if fetchResp.Cursor != "" {
			fmt.Printf("📄 Cursor for next page: %s\n", fetchResp.Cursor)
		}
	}

	// Now create a dataset reader to fetch the data back
	dataset := api.NewDataset(datasetInfo.ID)

	// Create a wrapper that implements eval.Dataset interface
	wrapper := &datasetWrapper{dataset: dataset}

	// Use the wrapper directly

	// Define a simple task that processes the input
	task := func(ctx context.Context, input string) (string, error) {
		// Simple example: just echo the input with a prefix
		result := fmt.Sprintf("Processed: %s", input)
		// fmt.Printf("🔄 Processing input: '%s' -> '%s'\n", input, result)
		return result, nil
	}

	// Create scorers
	scorers := []eval.Scorer[string, string]{
		autoevals.NewEquals[string, string](),
	}

	// Create and run the evaluation
	evaluation, err := eval.NewWithOpts(
		eval.Options{
			ProjectID:      project.ID,
			ExperimentName: "Dataset API Demo",
		},
		wrapper,
		task,
		scorers,
	)
	if err != nil {
		log.Fatalf("Failed to create evaluation: %v", err)
	}

	fmt.Println("\n🚀 Running evaluation with Braintrust dataset...")
	err = evaluation.Run()
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	fmt.Println("\n✅ Evaluation completed successfully!")
	fmt.Printf("🔗 View results at: http://localhost:3000/app/projects/%s\n", project.ID)
	fmt.Printf("📊 Dataset at: http://localhost:3000/app/projects/%s/datasets/%s\n", project.ID, datasetInfo.ID)
	fmt.Printf("🏷️  Project ID: %s\n", project.ID)
	fmt.Printf("📊 Dataset ID: %s\n", datasetInfo.ID)
}
