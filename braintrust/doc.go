// Package braintrust provides the core Braintrust SDK for Go.
//
// Braintrust is a platform for building reliable AI applications. This SDK provides
// tools for evaluation, experimentation, and observability of AI systems.
//
// # Quick Start
//
// To get started with evaluations:
//
//	import (
//		"context"
//		"log"
//		"github.com/braintrust/braintrust-x-go/braintrust/eval"
//		"github.com/braintrust/braintrust-x-go/braintrust/trace"
//	)
//
//	// Set up tracing (requires BRAINTRUST_API_KEY environment variable)
//	// export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
//	// Define your task function
//	myTask := func(ctx context.Context, input string) (string, error) {
//		if input == "Hello" {
//			return "greeting", nil
//		}
//		return "unknown", nil
//	}
//
//	// Define your scorer function
//	myScorer := func(ctx context.Context, input, expected, result string) (float64, error) {
//		if expected == result {
//			return 1.0, nil
//		}
//		return 0.0, nil
//	}
//
//	// Create an evaluation
//	evaluation, err := eval.NewWithOpts(
//		eval.Options{
//			ProjectName:    "my-project",
//			ExperimentName: "my-experiment",
//		},
//		[]eval.Case[string, string]{
//			{Input: "Hello", Expected: "greeting"},
//		},
//		myTask,
//		[]eval.Scorer[string, string]{
//			eval.NewScorer("accuracy", myScorer),
//		},
//	)
//
// # Tracing
//
// For automatic tracing of OpenAI calls:
//
//	import (
//		"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
//		"github.com/openai/openai-go/option"
//	)
//
//	client := openai.NewClient(
//		option.WithMiddleware(traceopenai.Middleware),
//	)
//
// # Configuration
//
// The SDK reads configuration from environment variables:
//   - BRAINTRUST_API_KEY: Your Braintrust API key
//   - BRAINTRUST_API_URL: API endpoint (defaults to https://www.braintrust.ai)
//   - BRAINTRUST_EXPERIMENT_ID: Default experiment ID for tracing
//
// # Learn More
//
// For more examples and documentation, visit:
//   - GitHub: https://github.com/braintrust/braintrust-x-go
//   - Documentation: https://www.braintrust.ai/docs
//   - Examples: See the examples/ directory in this repository
package braintrust
