// This example shows how to run an eval against a dataset downloaded
// from braintrust.dev.

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
	"github.com/braintrustdata/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

// QuestionInput represents the input structure for a question
type QuestionInput struct {
	Text     string `json:"text"`
	Context  string `json:"context"`
	Language string `json:"language"`
}

// AnswerExpected represents the expected output structure
type AnswerExpected struct {
	Response string `json:"response"`
}

// DatasetEvent represents the complete event structure from the dataset
type DatasetEvent struct {
	ID       string                 `json:"id,omitempty"`
	Input    QuestionInput          `json:"input"`
	Expected AnswerExpected         `json:"expected"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// initializeDataset creates a dataset with sample data and returns the dataset ID
func initializeDataset(projectID string) (string, error) {
	// Create a dataset with timestamp to make it unique
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	datasetInfo, err := api.CreateDataset(api.DatasetRequest{
		ProjectID:   projectID,
		Name:        "Sample Struct Dataset " + timestamp,
		Description: "A sample dataset for demonstrating struct-based evaluation",
	})
	if err != nil {
		return "", fmt.Errorf("failed to create dataset: %v", err)
	}
	fmt.Printf("📊 Created dataset: %s (ID: %s)\n", datasetInfo.Name, datasetInfo.ID)

	// Insert some sample data into the dataset
	sampleEvents := []api.DatasetEvent{
		{
			Input: map[string]interface{}{
				"text":     "hello world",
				"context":  "Basic capitalization",
				"language": "en",
			},
			Expected: map[string]interface{}{
				"response": "Hello World",
			},
		},
		{
			Input: map[string]interface{}{
				"text":     "braintrust is awesome",
				"context":  "Company name capitalization",
				"language": "en",
			},
			Expected: map[string]interface{}{
				"response": "Braintrust Is Awesome",
			},
		},
		{
			Input: map[string]interface{}{
				"text":     "artificial intelligence",
				"context":  "Technical term capitalization",
				"language": "en",
			},
			Expected: map[string]interface{}{
				"response": "Artificial Intelligence",
			},
		},
	}

	err = api.InsertDatasetEvents(datasetInfo.ID, sampleEvents)
	if err != nil {
		return "", fmt.Errorf("failed to insert events: %v", err)
	}
	fmt.Printf("📝 Inserted %d events into dataset\n", len(sampleEvents))

	// Verify the data was inserted by fetching it back
	fmt.Println("\n🔍 Querying dataset to verify data...")
	fetchResp, err := api.FetchDatasetEvents(datasetInfo.ID, api.DatasetFetchRequest{Limit: 10})
	if err != nil {
		log.Printf("Warning: Failed to fetch events for verification: %v", err)
	} else {
		fmt.Printf("📊 Found %d events in dataset\n", len(fetchResp.Events))
	}

	return datasetInfo.ID, nil
}

func main() {
	// Initialize OpenTelemetry tracing for Braintrust
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer teardown() // Ensure traces are flushed on exit

	// First, create a project
	project, err := api.RegisterProject("Struct Dataset Example Project")
	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}
	fmt.Printf("📁 Created project: %s (ID: %s)\n", project.Name, project.ID)

	// Initialize dataset
	datasetID, err := initializeDataset(project.ID)
	if err != nil {
		log.Fatalf("Failed to initialize dataset: %v", err)
	}

	// Create cases using eval.GetDatasetByID with separate Input/Expected types
	cases, err := eval.GetDatasetByID[QuestionInput, AnswerExpected](datasetID)
	if err != nil {
		log.Fatalf("❌ Failed to get dataset: %v", err)
	}

	// Define a task that processes the input and returns the expected structure
	task := func(ctx context.Context, input QuestionInput) (AnswerExpected, error) {
		// Simple example: capitalize the first letter of each word
		fmt.Printf("🔄 Processing text: '%s' (context: %s, language: %s)\n",
			input.Text, input.Context, input.Language)

		// Simple capitalization logic - capitalize first letter of each word
		words := strings.Fields(input.Text)
		capitalizedWords := make([]string, len(words))

		for i, word := range words {
			if len(word) > 0 {
				capitalizedWords[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			} else {
				capitalizedWords[i] = word
			}
		}

		response := strings.Join(capitalizedWords, " ")

		return AnswerExpected{
			Response: response,
		}, nil
	}

	// Create scorers that work with the business logic types
	scorers := []eval.Scorer[QuestionInput, AnswerExpected]{
		autoevals.NewEquals[QuestionInput, AnswerExpected](),
	}

	// Create and run the evaluation
	experimentID, err := eval.ResolveExperimentID("Capitalization Task Demo", project.ID)
	if err != nil {
		log.Fatalf("Failed to resolve experiment: %v", err)
	}

	evaluation := eval.New(experimentID, cases, task, scorers)

	fmt.Println("\n🚀 Running evaluation with struct-based dataset...")
	err = evaluation.Run(context.Background())
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	fmt.Println("\n✅ Evaluation completed successfully!")
	fmt.Printf("🔗 View results at: http://localhost:3000/app/projects/%s\n", project.ID)
	fmt.Printf("📊 Dataset at: http://localhost:3000/app/projects/%s/datasets/%s\n", project.ID, datasetID)
	fmt.Printf("🏷️  Project ID: %s\n", project.ID)
	fmt.Printf("📊 Dataset ID: %s\n", datasetID)
}
