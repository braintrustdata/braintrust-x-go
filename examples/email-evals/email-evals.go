// This is a realistic eval example.
package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/braintrust/braintrust-x-go/braintrust/eval"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
	"github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
)

var tracer = otel.Tracer("email-subject-optimizer")

// EmailCampaign represents the email marketing campaign context
type EmailCampaign struct {
	EmailContent   string `json:"email_content"`
	TargetAudience string `json:"target_audience"`
	CampaignType   string `json:"campaign_type"`
	BrandVoice     string `json:"brand_voice"`
	ProductName    string `json:"product_name"`
}

// SubjectLineResponse represents the generated subject line with metadata
type SubjectLineResponse struct {
	SubjectLine string `json:"subject_line"`
	Reasoning   string `json:"reasoning"`
	Urgency     string `json:"urgency"` // low, medium, high
}

func main() {
	log.Println("📧 Starting Email Subject Line Optimization Evaluation")
	log.Println("======================================================")

	client := openai.NewClient(
		option.WithMiddleware(traceopenai.Middleware),
	)

	teardown, err := trace.Quickstart()
	if err != nil {
		log.Fatalf("Error starting trace: %v", err)
	}
	defer teardown()

	// Subject line generation task
	generateSubjectLine := func(ctx context.Context, campaign EmailCampaign) (SubjectLineResponse, error) {
		_, span := tracer.Start(ctx, "custom_subject_generation")
		defer span.End()

		span.SetAttributes(
			attribute.String("campaign.type", campaign.CampaignType),
			attribute.String("campaign.audience", campaign.TargetAudience),
			attribute.String("campaign.brand_voice", campaign.BrandVoice),
			attribute.String("campaign.product", campaign.ProductName),
		)

		// Build context-aware prompt
		prompt := fmt.Sprintf(`Create an engaging email subject line for this marketing campaign:

Campaign Type: %s
Target Audience: %s
Brand Voice: %s
Product: %s

Email Content Preview: %s

Requirements:
- Maximum 50 characters (mobile-friendly)
- Match the %s brand voice
- Appeal to %s
- Drive opens and engagement
- Avoid spam trigger words

Respond with just the subject line, no quotes or explanations.`,
			campaign.CampaignType,
			campaign.TargetAudience,
			campaign.BrandVoice,
			campaign.ProductName,
			truncateText(campaign.EmailContent, 200),
			campaign.BrandVoice,
			campaign.TargetAudience)

		params := responses.ResponseNewParams{
			Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String("Generate a compelling, concise email subject line that maximizes open rates."),
		}

		resp, err := client.Responses.New(ctx, params)
		if err != nil {
			return SubjectLineResponse{}, fmt.Errorf("subject line generation failed: %w", err)
		}

		subjectLine := strings.TrimSpace(resp.OutputText())

		// Clean up any quotes that might be included
		subjectLine = strings.Trim(subjectLine, `"'`)

		response := SubjectLineResponse{
			SubjectLine: subjectLine,
			Reasoning:   "AI-generated based on campaign context",
			Urgency:     detectUrgency(subjectLine),
		}

		span.SetAttributes(
			attribute.String("output.subject_line", response.SubjectLine),
			attribute.Int("output.length", len(response.SubjectLine)),
			attribute.String("output.urgency", response.Urgency),
		)

		return response, nil
	}

	// Real email marketing scenarios
	testCases := []eval.Case[EmailCampaign, SubjectLineResponse]{
		{
			Input: EmailCampaign{
				EmailContent:   "Our biggest sale of the year is here! Save up to 70% on all premium software tools. Limited time offer - only 48 hours left. Featured products include project management suites, design tools, and productivity apps.",
				TargetAudience: "small business owners",
				CampaignType:   "flash_sale",
				BrandVoice:     "professional",
				ProductName:    "Software Bundle",
			},
			Expected: SubjectLineResponse{
				SubjectLine: "70% Off Software Bundle - 48hrs Only",
				Urgency:     "high",
			},
		},
		{
			Input: EmailCampaign{
				EmailContent:   "Introducing our new sustainable clothing line made from recycled materials. Each piece tells a story of environmental responsibility while delivering premium comfort and style.",
				TargetAudience: "environmentally conscious millennials",
				CampaignType:   "product_launch",
				BrandVoice:     "casual",
				ProductName:    "EcoWear Collection",
			},
			Expected: SubjectLineResponse{
				SubjectLine: "New EcoWear: Style Meets Sustainability",
				Urgency:     "low",
			},
		},
		{
			Input: EmailCampaign{
				EmailContent:   "Your free trial expires in 3 days. Upgrade now to keep access to premium features including advanced analytics, unlimited projects, and priority support.",
				TargetAudience: "free trial users",
				CampaignType:   "conversion",
				BrandVoice:     "helpful",
				ProductName:    "Premium Plan",
			},
			Expected: SubjectLineResponse{
				SubjectLine: "3 Days Left - Upgrade to Premium",
				Urgency:     "high",
			},
		},
		{
			Input: EmailCampaign{
				EmailContent:   "Weekly industry insights: AI adoption in healthcare reached 87% this quarter. New regulations for data privacy. Best practices for remote team management.",
				TargetAudience: "healthcare executives",
				CampaignType:   "newsletter",
				BrandVoice:     "authoritative",
				ProductName:    "Industry Report",
			},
			Expected: SubjectLineResponse{
				SubjectLine: "Healthcare AI Hits 87% + Privacy Updates",
				Urgency:     "low",
			},
		},
		{
			Input: EmailCampaign{
				EmailContent:   "Join us for an exclusive webinar with industry experts discussing the future of fintech. Learn about emerging trends, regulatory changes, and investment opportunities.",
				TargetAudience: "fintech professionals",
				CampaignType:   "event_invitation",
				BrandVoice:     "professional",
				ProductName:    "Fintech Future Webinar",
			},
			Expected: SubjectLineResponse{
				SubjectLine: "Exclusive: Future of Fintech Webinar",
				Urgency:     "medium",
			},
		},
	}

	scorers := []eval.Scorer[EmailCampaign, SubjectLineResponse]{
		// Length compliance scorer
		eval.NewScorer("length_compliance", func(ctx context.Context, input EmailCampaign, expected, result SubjectLineResponse) (float64, error) {
			length := len(result.SubjectLine)
			if length <= 50 {
				return 1.0, nil
			} else if length <= 60 {
				return 0.7, nil // Acceptable but not ideal
			}
			return 0.0, nil // Too long for mobile
		}),

		// AI-powered engagement prediction scorer
		eval.NewScorer("engagement_prediction", func(ctx context.Context, input EmailCampaign, _, result SubjectLineResponse) (float64, error) {
			_, span := tracer.Start(ctx, "custom_engagement_scoring")
			defer span.End()

			span.SetAttributes(
				attribute.String("scorer.type", "engagement_prediction"),
				attribute.String("subject_line", result.SubjectLine),
				attribute.String("campaign_type", input.CampaignType),
			)

			evalPrompt := fmt.Sprintf(`Rate this email subject line's engagement potential (0-10):

Subject Line: "%s"
Campaign Type: %s
Target Audience: %s
Brand Voice: %s

Evaluate based on:
1. Clarity and relevance
2. Emotional appeal
3. Urgency/curiosity factor
4. Mobile-friendliness
5. Spam avoidance

Consider: Will %s want to open this email?

Respond with only a number 0-10.`,
				result.SubjectLine,
				input.CampaignType,
				input.TargetAudience,
				input.BrandVoice,
				input.TargetAudience)

			params := responses.ResponseNewParams{
				Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(evalPrompt)},
				Model:        openai.ChatModelGPT4oMini,
				Instructions: openai.String("Return only a numeric engagement score from 0-10."),
			}

			resp, err := client.Responses.New(ctx, params)
			if err != nil {
				return 0.0, fmt.Errorf("engagement prediction failed: %w", err)
			}

			scoreText := strings.TrimSpace(resp.OutputText())
			var score float64
			if _, err := fmt.Sscanf(scoreText, "%f", &score); err != nil {
				score = 0
			}

			normalizedScore := score / 10.0

			span.SetAttributes(
				attribute.Float64("raw_engagement_score", score),
				attribute.Float64("normalized_score", normalizedScore),
			)

			return normalizedScore, nil
		}),

		// Spam filter risk scorer
		eval.NewScorer("spam_risk", func(ctx context.Context, input EmailCampaign, expected, result SubjectLineResponse) (float64, error) {
			spamTriggers := []string{
				"FREE", "URGENT", "ACT NOW", "LIMITED TIME", "CLICK HERE",
				"GUARANTEE", "NO OBLIGATION", "RISK FREE", "CASH", "MONEY",
				"!!!", "100%", "AMAZING", "INCREDIBLE", "UNBELIEVABLE",
			}

			subjectUpper := strings.ToUpper(result.SubjectLine)
			triggerCount := 0

			for _, trigger := range spamTriggers {
				if strings.Contains(subjectUpper, trigger) {
					triggerCount++
				}
			}

			// Check for excessive punctuation
			exclamationCount := strings.Count(result.SubjectLine, "!")
			if exclamationCount > 1 {
				triggerCount++
			}

			// Score: 1.0 = no spam risk, 0.0 = high spam risk
			switch triggerCount {
			case 0:
				return 1.0, nil
			case 1:
				return 0.7, nil
			case 2:
				return 0.4, nil
			}
			return 0.0, nil
		}),

		// Product mention scorer
		eval.NewScorer("product_relevance", func(ctx context.Context, input EmailCampaign, expected, result SubjectLineResponse) (float64, error) {
			subjectLower := strings.ToLower(result.SubjectLine)
			productLower := strings.ToLower(input.ProductName)

			// Check if product name or key terms are mentioned
			productWords := strings.Fields(productLower)
			mentionCount := 0

			for _, word := range productWords {
				if len(word) > 2 && strings.Contains(subjectLower, word) {
					mentionCount++
				}
			}

			if mentionCount > 0 {
				return 1.0, nil
			}

			// Check for campaign type relevance as backup
			campaignTerms := map[string][]string{
				"flash_sale":       {"sale", "off", "save", "deal"},
				"product_launch":   {"new", "introducing", "launch"},
				"conversion":       {"upgrade", "premium", "expires"},
				"newsletter":       {"update", "news", "insights"},
				"event_invitation": {"webinar", "event", "join"},
			}

			if terms, exists := campaignTerms[input.CampaignType]; exists {
				for _, term := range terms {
					if strings.Contains(subjectLower, term) {
						return 0.7, nil
					}
				}
			}

			return 0.3, nil
		}),
	}

	evaluation, err := eval.NewWithOpts(
		eval.Options{
			ProjectName:    "Email Marketing Optimization",
			ExperimentName: "Subject Line A/B Testing v1",
		},
		eval.NewCases(testCases), generateSubjectLine, scorers)

	if err != nil {
		log.Fatalf("❌ Failed to create evaluation: %v", err)
	}

	log.Println("🚀 Running email subject line evaluation...")
	err = evaluation.Run()
	if err != nil {
		log.Printf("⚠️  Evaluation completed with some issues: %v", err)
	} else {
		log.Println("✅ Email subject line evaluation completed successfully!")
	}
}

// Helper functions
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func detectUrgency(subjectLine string) string {
	urgentWords := []string{"urgent", "expires", "limited", "hours", "days", "now", "hurry", "last chance"}
	mediumWords := []string{"exclusive", "special", "new", "announcement"}

	subjectLower := strings.ToLower(subjectLine)

	// Check for numbers indicating time
	timePattern := regexp.MustCompile(`\d+\s*(hour|day|hr|min)`)
	if timePattern.MatchString(subjectLower) {
		return "high"
	}

	for _, word := range urgentWords {
		if strings.Contains(subjectLower, word) {
			return "high"
		}
	}

	for _, word := range mediumWords {
		if strings.Contains(subjectLower, word) {
			return "medium"
		}
	}

	return "low"
}
