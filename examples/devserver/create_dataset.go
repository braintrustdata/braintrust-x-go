package main

import (
	"context"
	"fmt"
	"log"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
)

func main() {
	ctx := context.Background()
	_ = ctx

	// Initialize Braintrust
	_, err := braintrust.Login()
	if err != nil {
		log.Fatalf("Failed to login: %v", err)
	}

	// Get or create project
	project, err := api.RegisterProject("go-sdk-examples")
	if err != nil {
		log.Fatalf("Failed to get project: %v", err)
	}
	fmt.Printf("Project: %s (ID: %s)\n", project.Name, project.ID)

	// Create dataset
	dataset, err := api.CreateDataset(api.DatasetRequest{
		ProjectID:   project.ID,
		Name:        "uppercase-test-data",
		Description: "Test data for uppercase evaluator",
	})
	if err != nil {
		log.Fatalf("Failed to create dataset: %v", err)
	}
	fmt.Printf("Dataset: %s (ID: %s)\n", dataset.Name, dataset.ID)

	// Add test cases
	testCases := []struct {
		input    string
		expected string
	}{
		{"hello", "HELLO"},
		{"world", "WORLD"},
		{"testing", "TESTING"},
		{"go lang", "GO LANG"},
		{"braintrust", "BRAINTRUST"},
	}

	events := make([]api.DatasetEvent, len(testCases))
	for i, tc := range testCases {
		events[i] = api.DatasetEvent{
			Input:    tc.input,
			Expected: tc.expected,
		}
		fmt.Printf("Added: %s -> %s\n", tc.input, tc.expected)
	}

	err = api.InsertDatasetEvents(dataset.ID, events)
	if err != nil {
		log.Fatalf("Failed to insert events: %v", err)
	}

	fmt.Println("\nDataset created successfully!")
	fmt.Printf("View it at: https://www.braintrust.dev/app/object?object_type=dataset&object_id=%s\n", dataset.ID)
}
