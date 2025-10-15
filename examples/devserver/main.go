// This example demonstrates the Braintrust dev server for remote evaluations.
package main

import (
	"context"
	"log"
	"strings"

	"github.com/braintrustdata/braintrust-x-go/braintrust/devserver"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
	"github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
	// Initialize tracing (required for eval.Run)
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer teardown()

	// Create dev server
	server := devserver.New(devserver.Config{
		Host: "localhost",
		Port: 8300,
	})

	// Register a simple string evaluation
	// This can be run from the Braintrust web UI with the "uppercase-test-data" dataset
	// (Dataset ID: 3551aeb3-8e41-4176-8897-7923be40f067)
	err = devserver.Register(server, devserver.RemoteEval[string, string]{
		Name:        "uppercase",
		ProjectName: "go-sdk-examples",
		Task: func(ctx context.Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		},
		Scorers: []eval.Scorer[string, string]{
			eval.NewScorer("length", func(ctx context.Context, input, expected, result string, meta eval.Metadata) (eval.Scores, error) {
				score := float64(len(result)) / 10.0
				if score > 1.0 {
					score = 1.0
				}
				return eval.S(score), nil
			}),
		},
	})
	if err != nil {
		log.Fatalf("Failed to register evaluator: %v", err)
	}

	// Start server (blocks)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
