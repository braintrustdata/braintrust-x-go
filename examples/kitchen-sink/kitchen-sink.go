// This example is used to test the UI and all the features it supports (OpenAI, errors, custom tracing, etc).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
)

var tracer = otel.Tracer("kitchen-sink-example")

func main() {
	log.Println("🧪 Starting Kitchen Sink Example - Testing All Repository Features")
	log.Println("================================================================")

	// Initialize OpenAI client with tracing middleware
	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	// Start distributed tracing
	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("❌ Error starting trace: %v", err)
	}
	defer teardown()

	// Run all examples
	runMathEval()
	runTextProcessingEval(client)
	runMixedScenarioEval(client)

	log.Println("✅ Kitchen Sink Example completed successfully!")
}

// runMathEval demonstrates basic eval functionality with no external dependencies
func runMathEval() {
	log.Println("\n📊 Running Math Evaluation (Basic Functionality)")
	log.Println("--------------------------------------------------")

	// Task that sometimes works, sometimes fails
	mathTask := func(_ context.Context, input int) (float64, error) {
		switch input {
		case 42:
			return 0, errors.New("universe error: cannot compute answer to everything")
		case 13:
			return 0, errors.New("superstition error: unlucky number")
		default:
			return math.Sqrt(float64(input)), nil
		}
	}

	// Mix of scorers - some pass, some fail
	scorers := []eval.Scorer[int, float64]{
		autoevals.NewEquals[int, float64](),
		eval.NewScorer("within_tolerance", func(_ context.Context, input int, expected, result float64) (float64, error) {
			if input == 16 {
				return 0, errors.New("tolerance checker malfunction")
			}
			diff := math.Abs(expected - result)
			if diff < 0.1 {
				return 1.0, nil
			}
			return 0.0, nil
		}),
		eval.NewScorer("is_positive", func(_ context.Context, _ int, _, result float64) (float64, error) {
			if result >= 0 {
				return 1.0, nil
			}
			return 0.0, nil
		}),
	}

	// Test cases with various scenarios
	cases := []eval.Case[int, float64]{
		{Input: 4, Expected: 2.0},  // ✅ Perfect match
		{Input: 9, Expected: 3.0},  // ✅ Perfect match
		{Input: 16, Expected: 4.0}, // ⚠️  Scorer fails (tolerance_checker malfunction)
		{Input: 25, Expected: 5.1}, // ⚠️  Close but not exact (tolerance should pass, equals should fail)
		{Input: 42, Expected: 6.5}, // ❌ Task fails (universe error)
		{Input: 13, Expected: 3.6}, // ❌ Task fails (superstition error)
	}

	evaluation, err := eval.NewWithOpts(
		eval.Options{
			ProjectName:    "Go Kitchen Sink Examples",
			ExperimentName: "Math Evaluation - Basic Functionality",
		},
		cases, mathTask, scorers)
	if err != nil {
		log.Fatalf("❌ Failed to create math evaluation: %v", err)
	}

	err = evaluation.Run()
	if err != nil {
		log.Printf("⚠️  Math evaluation completed with errors: %v", err)
	} else {
		log.Println("✅ Math evaluation completed successfully")
	}
}

// runTextProcessingEval demonstrates OpenAI integration with various failure modes
func runTextProcessingEval(client openai.Client) {
	log.Println("\n🤖 Running Text Processing Evaluation (OpenAI Integration)")
	log.Println("-----------------------------------------------------------")

	// Task using OpenAI that can fail in different ways - with custom tracing
	sentimentTask := func(ctx context.Context, text string) (string, error) {
		ctx, span := tracer.Start(ctx, "custom_task_span")
		defer span.End()

		span.SetAttributes(
			attribute.String("task.type", "sentiment_analysis"),
			attribute.Int("input.length", len(text)),
		)

		// Simulate various failure scenarios
		if strings.Contains(text, "BROKEN") {
			return "", errors.New("task preprocessing failed: broken input detected")
		}

		if strings.Contains(text, "TIMEOUT") {
			return "", errors.New("task timeout: request took too long")
		}

		prompt := fmt.Sprintf("Analyze the sentiment of this text and respond with only 'positive', 'negative', or 'neutral': %s", text)

		params := responses.ResponseNewParams{
			Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String("Respond with exactly one word: positive, negative, or neutral"),
		}

		resp, err := client.Responses.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("openai request failed: %w", err)
		}

		result := strings.ToLower(strings.TrimSpace(resp.OutputText()))

		// Simulate occasional model confusion
		if strings.Contains(text, "CONFUSE") {
			return "unknown", nil // Wrong format to trigger scorer errors
		}

		return result, nil
	}

	// Scorers with different failure scenarios
	scorers := []eval.Scorer[string, string]{
		autoevals.NewEquals[string, string](),
		eval.NewScorer("valid_sentiment", func(ctx context.Context, input, _, result string) (float64, error) {
			_, span := tracer.Start(ctx, "custom_score_span")
			defer span.End()

			span.SetAttributes(
				attribute.String("scorer.name", "valid_sentiment"),
				attribute.String("result.sentiment", result),
			)

			validSentiments := map[string]bool{
				"positive": true,
				"negative": true,
				"neutral":  true,
			}

			if strings.Contains(input, "SCORER_FAIL") {
				return 0, errors.New("sentiment validator crashed: internal error")
			}

			if validSentiments[result] {
				return 1.0, nil
			}
			return 0.0, nil
		}),
		eval.NewScorer("sentiment_agreement", func(_ context.Context, input, expected, result string) (float64, error) {
			// More lenient scorer that gives partial credit
			if strings.Contains(input, "PARTIAL_SCORER_FAIL") {
				return 0, errors.New("agreement checker malfunction: cannot determine agreement")
			}

			// Both positive sentiments or both negative sentiments get partial credit
			positiveWords := []string{"positive"}
			negativeWords := []string{"negative"}

			expectedPositive := contains(positiveWords, expected)
			resultPositive := contains(positiveWords, result)
			expectedNegative := contains(negativeWords, expected)
			resultNegative := contains(negativeWords, result)

			if expected == result {
				return 1.0, nil // Perfect match
			} else if (expectedPositive && resultPositive) || (expectedNegative && resultNegative) {
				return 0.7, nil // Partial credit
			}
			return 0.0, nil
		}),
	}

	cases := []eval.Case[string, string]{
		{Input: "I love this product!", Expected: "positive"},             // ✅ Should work perfectly
		{Input: "This is terrible and I hate it", Expected: "negative"},   // ✅ Should work perfectly
		{Input: "This is okay, nothing special", Expected: "neutral"},     // ✅ Should work perfectly
		{Input: "I love this CONFUSE product!", Expected: "positive"},     // ⚠️  Task returns "unknown", scorers should fail
		{Input: "BROKEN input text", Expected: "neutral"},                 // ❌ Task fails (preprocessing)
		{Input: "This will TIMEOUT surely", Expected: "negative"},         // ❌ Task fails (timeout)
		{Input: "I hate this SCORER_FAIL thing", Expected: "negative"},    // ⚠️  Task works, valid_sentiment scorer fails
		{Input: "Good product PARTIAL_SCORER_FAIL", Expected: "positive"}, // ⚠️  Task works, sentiment_agreement scorer fails
	}

	evaluation, err := eval.NewWithOpts(
		eval.Options{
			ProjectName:    "Go Kitchen Sink Examples",
			ExperimentName: "Text Processing - OpenAI Integration",
		},
		cases, sentimentTask, scorers)
	if err != nil {
		log.Fatalf("❌ Failed to create text evaluation: %v", err)
	}

	err = evaluation.Run()
	if err != nil {
		log.Printf("⚠️  Text evaluation completed with errors: %v", err)
	} else {
		log.Println("✅ Text evaluation completed successfully")
	}
}

// runMixedScenarioEval demonstrates complex scenarios with multiple types of failures
func runMixedScenarioEval(client openai.Client) {
	log.Println("\n🎯 Running Mixed Scenario Evaluation (Complex Interactions)")
	log.Println("------------------------------------------------------------")

	// Complex task that combines OpenAI calls with local processing
	questionAnswerTask := func(ctx context.Context, question string) (string, error) {
		// Local preprocessing that can fail
		if len(question) < 5 {
			return "", errors.New("preprocessing failed: question too short")
		}

		if strings.Contains(question, "INVALID") {
			return "", errors.New("preprocessing failed: invalid characters detected")
		}

		// OpenAI call for question answering
		prompt := fmt.Sprintf("Answer this question concisely in one sentence: %s", question)

		params := responses.ResponseNewParams{
			Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String("Provide a concise, factual answer in one sentence."),
		}

		resp, err := client.Responses.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("llm call failed: %w", err)
		}

		answer := resp.OutputText()

		// Post-processing that can fail
		if strings.Contains(question, "POSTPROCESS_FAIL") {
			return "", errors.New("postprocessing failed: output validation error")
		}

		if len(answer) == 0 {
			return "", errors.New("postprocessing failed: empty response from model")
		}

		return answer, nil
	}

	// Complex scoring with multiple failure modes
	scorers := []eval.Scorer[string, string]{
		eval.NewScorer("length_check", func(ctx context.Context, input, expected, result string) (float64, error) {
			if strings.Contains(input, "LENGTH_SCORER_FAIL") {
				return 0, errors.New("length checker crashed: memory allocation failed")
			}

			// Penalize answers that are too short or too long
			if len(result) < 10 {
				return 0.3, nil
			} else if len(result) > 200 {
				return 0.5, nil
			}
			return 1.0, nil
		}),
		eval.NewScorer("keyword_relevance", func(ctx context.Context, input, expected, result string) (float64, error) {
			if strings.Contains(input, "RELEVANCE_SCORER_FAIL") {
				return 0, errors.New("relevance checker error: unable to analyze keywords")
			}

			// Extract key words from question and check if they appear in answer
			questionWords := strings.Fields(strings.ToLower(input))
			answerLower := strings.ToLower(result)

			matches := 0
			for _, word := range questionWords {
				if len(word) > 3 && strings.Contains(answerLower, word) {
					matches++
				}
			}

			if len(questionWords) == 0 {
				return 0.0, nil
			}

			relevanceScore := float64(matches) / float64(len(questionWords))
			return math.Min(relevanceScore, 1.0), nil
		}),
		eval.NewScorer("contains_expected_info", func(ctx context.Context, input, expected, result string) (float64, error) {
			if strings.Contains(input, "INFO_SCORER_FAIL") {
				return 0, errors.New("information checker failed: semantic analysis unavailable")
			}

			// Simple check if expected information appears in result
			expectedLower := strings.ToLower(expected)
			resultLower := strings.ToLower(result)

			expectedWords := strings.Fields(expectedLower)
			matchedWords := 0

			for _, word := range expectedWords {
				if len(word) > 2 && strings.Contains(resultLower, word) {
					matchedWords++
				}
			}

			if len(expectedWords) == 0 {
				return 1.0, nil
			}

			return float64(matchedWords) / float64(len(expectedWords)), nil
		}),
	}

	cases := []eval.Case[string, string]{
		{
			Input:    "What is the capital of France?",
			Expected: "Paris is the capital of France",
		}, // ✅ Should work well
		{
			Input:    "How does photosynthesis work?",
			Expected: "Plants convert sunlight into energy using chlorophyll",
		}, // ✅ Should work well
		{
			Input:    "Why LENGTH_SCORER_FAIL is the sky blue?",
			Expected: "Light scattering causes blue sky appearance",
		}, // ⚠️  Task works, length_scorer fails
		{
			Input:    "What RELEVANCE_SCORER_FAIL causes rain?",
			Expected: "Water vapor condenses in clouds forming rain",
		}, // ⚠️  Task works, relevance_scorer fails
		{
			Input:    "How INFO_SCORER_FAIL do birds fly?",
			Expected: "Birds use wings and hollow bones for flight",
		}, // ⚠️  Task works, info_scorer fails
		{
			Input:    "INVALID characters @#$%",
			Expected: "Invalid input should be handled",
		}, // ❌ Task fails (preprocessing)
		{
			Input:    "Hi?",
			Expected: "Too short",
		}, // ❌ Task fails (too short)
		{
			Input:    "What happens when POSTPROCESS_FAIL occurs?",
			Expected: "Processing should handle errors gracefully",
		}, // ❌ Task fails (postprocessing)
	}

	evaluation, err := eval.NewWithOpts(
		eval.Options{
			ProjectName:    "Go Kitchen Sink Examples",
			ExperimentName: "Mixed Scenarios - Complex Interactions",
		},
		cases, questionAnswerTask, scorers)
	if err != nil {
		log.Fatalf("❌ Failed to create mixed scenario evaluation: %v", err)
	}

	err = evaluation.Run()

	if err != nil {
		log.Printf("⚠️  Mixed scenario evaluation completed with errors: %v", err)
	} else {
		log.Println("✅ Mixed scenario evaluation completed successfully")
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
