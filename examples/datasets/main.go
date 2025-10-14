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
	fmt.Printf("📝 Inserted %d events\n", len(sampleEvents))

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
	project, err := api.RegisterProject("go-sdk-examples")
	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}
	fmt.Printf("📁 Created project: %s (ID: %s)\n", project.Name, project.ID)

	// Initialize dataset
	datasetID, err := initializeDataset(project.ID)
	if err != nil {
		log.Fatalf("Failed to initialize dataset: %v", err)
	}

	fmt.Println("\n🚀 Running evaluation (limiting to 2 of 3 rows)...")
	_, err = eval.Run(context.Background(), eval.Opts[QuestionInput, AnswerExpected]{
		ProjectID:    project.ID,
		Experiment:   "Capitalization Task Demo",
		DatasetID:    datasetID, // Use DatasetID directly - eval.Run handles fetching
		DatasetLimit: 2,         // Only evaluate the first 2 rows
		Task: func(ctx context.Context, input QuestionInput) (AnswerExpected, error) {
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
		},
		Scorers: []eval.Scorer[QuestionInput, AnswerExpected]{
			autoevals.NewEquals[QuestionInput, AnswerExpected](),
		},
	})
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
}
