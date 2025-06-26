// Kitchen sink throws a bunch of scenarios that exercise all the conditions of the UI (custom tracing, errors, openai, etc)
// and is useful to spot check the UI.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/braintrust/braintrust-x-go/braintrust"
	"github.com/braintrust/braintrust-x-go/braintrust/api"
	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
)

var tracer = otel.Tracer("kitchen-sink-example")

func main() {
	log.Println("🧪 Starting Kitchen Sink Example")

	// List available functions first
	log.Println("📋 Listing available functions...")
	functions, err := api.ListFunctions(nil)
	if err != nil {
		log.Printf("⚠️ Failed to list functions: %v", err)
	} else {
		log.Printf("📋 Found %d functions:", len(functions.Objects))
		for _, fn := range functions.Objects {
			log.Printf("  - %s (ID: %s, Project: %s)", fn.Name, fn.ID, fn.ProjectID)
		}
	}

	// Initialize OpenAI client with tracing middleware
	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	// Get or create the project first to set as default
	project, err := api.RegisterProject("Kitchen Sink")
	if err != nil {
		log.Fatalf("❌ Error registering project: %v", err)
	}

	// Start distributed tracing with default project ID
	teardown, err := trace.Quickstart(braintrust.WithDefaultProjectID(project.ID))
	if err != nil {
		log.Fatalf("❌ Error starting trace: %v", err)
	}
	defer teardown()

	// Create sample dataset
	datasetID, err := createSampleDataset(project.ID)
	if err != nil {
		log.Fatalf("❌ Failed to create dataset: %v", err)
	}
	log.Printf("✅ Created dataset: %s", datasetID)

	// Run evaluations
	runKitchenSinkEval(client)
	runDatasetEval(client, datasetID)

	log.Println("✅ Kitchen Sink Example completed successfully!")
}

func runKitchenSinkEval(client openai.Client) {
	log.Println("🔥 Running Kitchen Sink Eval")

	// Task with custom tracing that sometimes fails
	task := func(ctx context.Context, input string) (string, error) {
		ctx, span := tracer.Start(ctx, "kitchen_sink_task")
		defer span.End()

		span.SetAttributes(
			attribute.String("input.text", input),
			attribute.Int("input.length", len(input)),
		)

		// Task errors
		if strings.Contains(input, "TASK_FAIL") {
			return "", errors.New("task failed: broken input")
		}

		// LLM calls
		if strings.Contains(input, "sentiment") {
			prompt := fmt.Sprintf("What's the sentiment of: %s. Reply with just positive/negative/neutral", input)
			params := responses.ResponseNewParams{
				Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
				Model:        openai.ChatModelGPT4oMini,
				Instructions: openai.String("Reply with one word only"),
			}
			resp, err := client.Responses.New(ctx, params)
			if err != nil {
				return "", fmt.Errorf("llm failed: %w", err)
			}
			return strings.ToLower(strings.TrimSpace(resp.OutputText())), nil
		}

		if strings.Contains(input, "capital") {
			prompt := fmt.Sprintf("What's the capital of %s? Just the city name.", strings.ReplaceAll(input, "capital of ", ""))
			params := responses.ResponseNewParams{
				Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
				Model:        openai.ChatModelGPT4oMini,
				Instructions: openai.String("Reply with just the city name"),
			}
			resp, err := client.Responses.New(ctx, params)
			if err != nil {
				return "", fmt.Errorf("llm failed: %w", err)
			}
			return strings.TrimSpace(resp.OutputText()), nil
		}

		// Simple cases
		return input, nil
	}

	scorers := []eval.Scorer[string, string]{
		// Autoevals
		autoevals.NewEquals[string, string](),
		autoevals.NewLevenshtein[string](),

		// Multi-score scorer with custom tracing
		eval.NewScorer("quality_check", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
			_, span := tracer.Start(ctx, "quality_scorer")
			defer span.End()

			span.SetAttributes(
				attribute.String("result.value", result),
				attribute.Int("result.length", len(result)),
			)

			// Scorer error
			if strings.Contains(input, "SCORER_FAIL") {
				return nil, errors.New("quality checker crashed")
			}

			// Multiple scores
			return eval.Scores{
				{Name: "length_ok", Score: func() float64 {
					if len(result) > 0 && len(result) < 100 {
						return 1.0
					}
					return 0.0
				}()},
				{Name: "not_empty", Score: func() float64 {
					if len(result) > 0 {
						return 1.0
					}
					return 0.0
				}()},
			}, nil
		}),
	}

	cases := []eval.Case[string, string]{
		// Success cases
		{Input: "hello", Expected: "hello", Tags: []string{"simple", "success"}},
		{Input: "sentiment: I love this!", Expected: "positive", Tags: []string{"llm", "sentiment"}},
		{Input: "capital of France", Expected: "Paris", Tags: []string{"llm", "geography"}},

		// Levenshtein test cases - these will fail Equals but score well on Levenshtein
		{Input: "hello world", Expected: "helo world", Tags: []string{"typo", "levenshtein"}}, // Missing 'l' - high similarity
		{Input: "testing", Expected: "tesing", Tags: []string{"typo", "levenshtein"}},         // Missing 't' - high similarity
		{Input: "algorithm", Expected: "algoritm", Tags: []string{"typo", "levenshtein"}},     // Missing 'h' - high similarity
		{Input: "programming", Expected: "programing", Tags: []string{"typo", "levenshtein"}}, // Missing 'm' - high similarity

		// Error cases
		{Input: "TASK_FAIL this", Expected: "anything", Tags: []string{"task_error"}},
		{Input: "SCORER_FAIL test", Expected: "test", Tags: []string{"scorer_error"}},

		// Mixed
		{Input: "maybe", Expected: "perhaps", Tags: []string{"mismatch"}},
		{Input: "completely", Expected: "different", Tags: []string{"mismatch", "low_similarity"}}, // Low Levenshtein score
	}

	experimentID, err := eval.ResolveProjectExperimentID("Kitchen Sink", "Kitchen Sink")
	if err != nil {
		log.Fatalf("❌ Failed to resolve experiment: %v", err)
	}

	evaluation := eval.New(experimentID, eval.NewCases(cases), task, scorers)
	err = evaluation.Run(context.Background())
	if err != nil {
		log.Printf("⚠️  Eval completed with errors: %v", err)
	} else {
		log.Println("✅ Eval completed")
	}
}

func createSampleDataset(projectID string) (string, error) {
	req := api.DatasetRequest{
		ProjectID: projectID,
		Name:      "Kitchen Sink QA",
	}

	dataset, err := api.CreateDataset(req)
	if err != nil {
		return "", fmt.Errorf("failed to create dataset: %w", err)
	}

	events := []api.DatasetEvent{
		{Input: "What is the capital of Japan?", Expected: "Tokyo"},
		{Input: "What is 5 + 3?", Expected: "8"},
		{Input: "What color is grass?", Expected: "green"},
	}

	err = api.InsertDatasetEvents(dataset.ID, events)
	if err != nil {
		return "", fmt.Errorf("failed to insert events: %w", err)
	}

	return dataset.ID, nil
}

func runDatasetEval(client openai.Client, datasetID string) {
	log.Printf("📊 Running Dataset Eval with %s", datasetID)

	task := func(ctx context.Context, question string) (string, error) {
		prompt := fmt.Sprintf("Answer this question briefly: %s", question)
		params := responses.ResponseNewParams{
			Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String("Give a short, accurate answer"),
		}

		resp, err := client.Responses.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("llm failed: %w", err)
		}

		return strings.TrimSpace(resp.OutputText()), nil
	}

	scorers := []eval.Scorer[string, string]{
		autoevals.NewEquals[string, string](),
		eval.NewScorer("contains_answer", func(_ context.Context, _, expected, result string, _ eval.Metadata) (eval.Scores, error) {
			score := 0.0
			if strings.Contains(strings.ToLower(result), strings.ToLower(expected)) {
				score = 1.0
			}
			return eval.Scores{{Name: "contains_answer", Score: score}}, nil
		}),
		eval.NewScorer("llm_judge", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
			prompt := fmt.Sprintf(`Rate how well this answer matches the expected answer on a scale of 0 to 1:

Question: %s
Expected: %s
Actual: %s

Reply with just a decimal number between 0 and 1.`, input, expected, result)

			params := responses.ResponseNewParams{
				Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
				Model:        openai.ChatModelGPT4oMini,
				Instructions: openai.String("Reply with just a decimal number from 0 to 1"),
			}

			resp, err := client.Responses.New(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("llm judge failed: %w", err)
			}

			scoreText := strings.TrimSpace(resp.OutputText())
			var score float64
			if _, err := fmt.Sscanf(scoreText, "%f", &score); err != nil {
				score = 0.0 // Default to 0 if parsing fails
			}

			// Clamp to 0-1 range
			if score > 1.0 {
				score = 1.0
			}
			if score < 0.0 {
				score = 0.0
			}

			return eval.Scores{{Name: "llm_judge", Score: score}}, nil
		}),
	}

	cases := eval.QueryDataset[string, string](datasetID)

	experimentID, err := eval.ResolveProjectExperimentID("Dataset QA", "Kitchen Sink")
	if err != nil {
		log.Fatalf("❌ Failed to resolve experiment: %v", err)
	}

	evaluation := eval.New(experimentID, cases, task, scorers)
	err = evaluation.Run(context.Background())
	if err != nil {
		log.Printf("⚠️  Dataset eval completed with errors: %v", err)
	} else {
		log.Println("✅ Dataset eval completed")
	}
}
