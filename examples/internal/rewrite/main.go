package main

import (
	"context"
	"log"
	"os"

	braintrust "github.com/braintrustdata/braintrust-x-go"
	"github.com/braintrustdata/braintrust-x-go/logger"
)

func main() {
	// Create a custom logger to see debug output
	customLogger := logger.NewDefaultLogger()

	// Create Braintrust client with configuration
	bt, err := braintrust.New(
		braintrust.WithAPIKey(os.Getenv("BRAINTRUST_API_KEY")),
		braintrust.WithProject("rewrite-test"),
		braintrust.WithLogger(customLogger),
		braintrust.WithTracingEnabled(true),
		braintrust.WithBlockingLogin(true), // async login
	)
	if err != nil {
		log.Fatalf("Failed to create Braintrust client: %v", err)
	}
	defer func() {
		if err := bt.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown Braintrust client: %v", err)
		}
	}()

	log.Println("Braintrust client created successfully!")
	log.Println()
	log.Println(bt)
	log.Println()

	// TODO: Add API and Eval usage examples here

	log.Println("Example complete")
}
