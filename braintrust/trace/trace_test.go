package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
)

// withSpanProcessor is a test-only option for setting a custom span processor
func withSpanProcessor(processor sdktrace.SpanProcessor) braintrust.Option {
	return func(c *braintrust.Config) {
		c.SpanProcessor = processor
	}
}

func newProjectIDParent(projectID string) Parent {
	return Parent{Type: ParentTypeProjectID, ID: projectID}
}

// Helper functions for testing
func flushOne(t *testing.T, exporter *tracetest.InMemoryExporter) tracetest.SpanStub {
	t.Helper()
	spans := exporter.GetSpans()
	exporter.Reset()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}
	return spans[0]
}

func attrEquals(span tracetest.SpanStub, key, expectedValue string) bool {
	for _, attr := range span.Attributes {
		if string(attr.Key) == key && attr.Value.AsString() == expectedValue {
			return true
		}
	}
	return false
}

func assertAttrEquals(t *testing.T, span tracetest.SpanStub, key, expectedValue string) {
	t.Helper()
	assert.True(t, attrEquals(span, key, expectedValue))
}

func TestSpanProcessor(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("12345"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:12345")

	// Assert we use the parent from the context if it is set.
	ctx := context.Background()
	ctx = SetParent(ctx, newProjectIDParent("67890"))
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	_ = tp.ForceFlush(context.Background())
	span = flushOne(t, exporter)
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:67890")

	// assert that if a span already has a parent, it is not overridden
	ctx = context.Background()
	ctx = SetParent(ctx, newProjectIDParent("77777"))
	_, span4 := tracer.Start(ctx, "test", trace.WithAttributes(attribute.String(ParentOtelAttrKey, "project_id:88888")))
	span4.End()
	_ = tp.ForceFlush(context.Background())
	span = flushOne(t, exporter)
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:88888")
}
func TestSpanProcessorNoDefaultProjectID(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Don't set a default project - should use the fallback
	err := Enable(tp,
		braintrust.WithDefaultProject(""), // Empty project name
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the fallback default parent
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_name:go-otel-default-project")
}

func TestSpanProcessorWithDefaultProjectName(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProject("12345"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Assert we use the default parent if none is set.
	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_name:12345")
}

func TestEnableErrors(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	err := Enable(tp, braintrust.WithAPIURL("invalid-url"))
	assert.Error(t, err)
}

func TestEnableOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []braintrust.Option
		wantAttr string
	}{
		{"project id", []braintrust.Option{braintrust.WithDefaultProjectID("test-123")}, "project_id:test-123"},
		{"project name", []braintrust.Option{braintrust.WithDefaultProject("test-name")}, "project_name:test-name"},
		{"no project", []braintrust.Option{}, "project_name:default-go-project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			exporter := tracetest.NewInMemoryExporter()
			processor := sdktrace.NewSimpleSpanProcessor(exporter)
			opts := append(tt.opts, withSpanProcessor(processor))

			err := Enable(tp, opts...)
			assert.NoError(t, err)

			_, span := tp.Tracer("test").Start(context.Background(), "test-span")
			span.End()
			err = tp.ForceFlush(context.Background())
			assert.NoError(t, err)

			span1 := flushOne(t, exporter)
			assertAttrEquals(t, span1, ParentOtelAttrKey, tt.wantAttr)
		})
	}
}

func TestEnableWithExistingProcessors(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	m1 := tracetest.NewInMemoryExporter()

	// Add one "existing" processor (e.g. a customer's APM processor)
	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(m1))

	// Add Braintrust's span processor
	m2 := tracetest.NewInMemoryExporter()
	processor2 := sdktrace.NewSimpleSpanProcessor(m2)
	err := Enable(tp,
		braintrust.WithDefaultProjectID("test-project-existing"),
		withSpanProcessor(processor2),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span-existing")
	span.End()

	err = tp.ForceFlush(context.Background())
	assert.NoError(err)

	span1, span2 := flushOne(t, m1), flushOne(t, m2)
	assert.Equal("test-span-existing", span1.Name)
	assert.Equal("test-span-existing", span2.Name)
	assertAttrEquals(t, span1, ParentOtelAttrKey, "project_id:test-project-existing")
	assertAttrEquals(t, span2, ParentOtelAttrKey, "project_id:test-project-existing")
}

func TestQuickstart(t *testing.T) {
	original := otel.GetTracerProvider()
	defer otel.SetTracerProvider(original)

	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	teardown, err := Quickstart(
		braintrust.WithDefaultProject("test-quickstart"),
		withSpanProcessor(processor),
	)
	assert.NoError(t, err)
	assert.NotNil(t, teardown)

	tp := otel.GetTracerProvider()
	assert.NotEqual(t, original, tp)

	// Test that we can create spans with the global tracer
	tracer := otel.Tracer("test-tracer")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	capturedSpan := spans[0]
	assert.Equal(t, "test-span", capturedSpan.Name)
	assertAttrEquals(t, capturedSpan, ParentOtelAttrKey, "project_name:test-quickstart")

	teardown()
}

func TestSpanFilterFunc_WithAttributes(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Custom filter that keeps spans with "important" attribute
	customFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "importance" && attr.Value.AsString() == "high" {
				return 1 // Keep
			}
			if string(attr.Key) == "noise" && attr.Value.AsBool() {
				return -1 // Drop
			}
		}
		return 0 // Don't influence
	}

	err := Enable(tp,
		braintrust.WithDefaultProjectID("filter-test"),
		braintrust.WithSpanFilterFuncs(customFilter),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Span with high importance - should be kept
	_, span1 := tracer.Start(context.Background(), "important-operation", trace.WithAttributes(
		attribute.String("importance", "high"),
	))
	span1.End()

	// Span marked as noise - should be dropped
	_, span2 := tracer.Start(context.Background(), "noisy-operation", trace.WithAttributes(
		attribute.Bool("noise", true),
	))
	span2.End()

	// Span with no special attributes - should go through other filters
	_, span3 := tracer.Start(context.Background(), "normal-operation")
	span3.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Should have 2 spans: important-operation (kept) and normal-operation (no influence = kept by default)
	// noisy-operation should be dropped
	assert.Len(spans, 2)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "important-operation")
	assert.Contains(spanNames, "normal-operation")
	assert.NotContains(spanNames, "noisy-operation")
}

func TestSpanProcessor_FilteringWithMultipleProcessors(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()

	// Add a non-Braintrust processor that should receive all spans
	allSpansExporter := tracetest.NewInMemoryExporter()
	allSpansProcessor := sdktrace.NewSimpleSpanProcessor(allSpansExporter)
	tp.RegisterSpanProcessor(allSpansProcessor)

	// Add Braintrust processor with filtering using Enable
	braintrustExporter := tracetest.NewInMemoryExporter()
	braintrustProcessor := sdktrace.NewSimpleSpanProcessor(braintrustExporter)

	dropNoisyFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "noise" && attr.Value.AsBool() {
				return -1 // Drop
			}
		}
		return 0 // Don't influence
	}

	err := Enable(tp,
		braintrust.WithDefaultProjectID("multi-processor-test"),
		braintrust.WithSpanFilterFuncs(dropNoisyFilter),
		withSpanProcessor(braintrustProcessor), // Use test helper to inject our test exporter
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Create spans
	_, span1 := tracer.Start(context.Background(), "important-operation")
	span1.End()

	_, span2 := tracer.Start(context.Background(), "noisy-operation", trace.WithAttributes(
		attribute.Bool("noise", true),
	))
	span2.End()

	_, span3 := tracer.Start(context.Background(), "normal-operation")
	span3.End()

	_ = tp.ForceFlush(context.Background())

	// All spans processor should receive all 3 spans
	allSpans := allSpansExporter.GetSpans()
	assert.Len(allSpans, 3)

	allSpanNames := make([]string, len(allSpans))
	for i, span := range allSpans {
		allSpanNames[i] = span.Name
	}
	assert.Contains(allSpanNames, "important-operation")
	assert.Contains(allSpanNames, "noisy-operation")
	assert.Contains(allSpanNames, "normal-operation")

	// Braintrust processor should only receive 2 spans (noisy-operation dropped)
	braintrustSpans := braintrustExporter.GetSpans()
	assert.Len(braintrustSpans, 2)

	braintrustSpanNames := make([]string, len(braintrustSpans))
	for i, span := range braintrustSpans {
		braintrustSpanNames[i] = span.Name
	}
	assert.Contains(braintrustSpanNames, "important-operation")
	assert.Contains(braintrustSpanNames, "normal-operation")
	assert.NotContains(braintrustSpanNames, "noisy-operation")
}

func TestAISpanFilterFunc_WithAIPrefixes(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Test the aiSpanFilterFunc directly
	err := Enable(tp,
		braintrust.WithDefaultProjectID("ai-filter-test"),
		braintrust.WithSpanFilterFuncs(aiSpanFilterFunc),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Spans with AI-related attributes - should be kept
	_, span1 := tracer.Start(context.Background(), "openai-call", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("gen_ai.request.model", "gpt-4"),
	))
	span1.End()

	_, span2 := tracer.Start(context.Background(), "braintrust-log", trace.WithAttributes(
		attribute.String("braintrust.log.id", "12345"),
	))
	span2.End()

	_, span3 := tracer.Start(context.Background(), "llm-call", trace.WithAttributes(
		attribute.String("llm.request.type", "completion"),
	))
	span3.End()

	_, span4 := tracer.Start(context.Background(), "ai-service", trace.WithAttributes(
		attribute.String("ai.model.name", "claude"),
	))
	span4.End()

	_, span5 := tracer.Start(context.Background(), "traceloop-span", trace.WithAttributes(
		attribute.String("traceloop.span.kind", "llm"),
	))
	span5.End()

	// Non-AI spans - should be filtered out (aiSpanFilterFunc returns 0)
	_, span6 := tracer.Start(context.Background(), "database-query", trace.WithAttributes(
		attribute.String("db.statement", "SELECT * FROM users"),
	))
	span6.End()

	_, span7 := tracer.Start(context.Background(), "http-request")
	span7.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Should only have the 5 AI-related spans
	assert.Len(spans, 5)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "openai-call")
	assert.Contains(spanNames, "braintrust-log")
	assert.Contains(spanNames, "llm-call")
	assert.Contains(spanNames, "ai-service")
	assert.Contains(spanNames, "traceloop-span")
	assert.NotContains(spanNames, "database-query")
	assert.NotContains(spanNames, "http-request")
}

func TestAISpanFilterFunc_WithSpanNames(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("ai-name-filter-test"),
		braintrust.WithSpanFilterFuncs(aiSpanFilterFunc),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Spans with AI-related names - should be kept
	_, span1 := tracer.Start(context.Background(), "gen_ai.completion")
	span1.End()

	_, span2 := tracer.Start(context.Background(), "braintrust.experiment.log")
	span2.End()

	_, span3 := tracer.Start(context.Background(), "llm.chat_completion")
	span3.End()

	// Non-AI span names - should be filtered out
	_, span4 := tracer.Start(context.Background(), "user.login")
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Should only have the 3 AI-related spans
	assert.Len(spans, 3)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "gen_ai.completion")
	assert.Contains(spanNames, "braintrust.experiment.log")
	assert.Contains(spanNames, "llm.chat_completion")
	assert.NotContains(spanNames, "user.login")
}

func TestWithFilterAISpans_Option(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Use the WithFilterAISpans option
	err := Enable(tp,
		braintrust.WithDefaultProjectID("ai-option-test"),
		braintrust.WithFilterAISpans(true),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// AI-related spans - should be kept
	_, span1 := tracer.Start(context.Background(), "gen_ai.completion", trace.WithAttributes(
		attribute.String("gen_ai.request.model", "gpt-4"),
	))
	span1.End()

	_, span2 := tracer.Start(context.Background(), "normal-http-call", trace.WithAttributes(
		attribute.String("llm.provider", "openai"),
	))
	span2.End()

	// Non-AI spans - should be filtered out
	_, span3 := tracer.Start(context.Background(), "database-query", trace.WithAttributes(
		attribute.String("db.statement", "SELECT * FROM users"),
	))
	span3.End()

	_, span4 := tracer.Start(context.Background(), "http.request")
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Should only have the 2 AI-related spans
	assert.Len(spans, 2)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "gen_ai.completion")
	assert.Contains(spanNames, "normal-http-call") // kept because of llm.provider attribute
	assert.NotContains(spanNames, "database-query")
	assert.NotContains(spanNames, "http.request")
}

func TestWithFilterAISpans_CombinedWithCustomFilters(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Custom filter that keeps "important" spans and drops "low" importance
	customFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "importance" {
				switch attr.Value.AsString() {
				case "high":
					return 1 // Keep
				case "low":
					return -1 // Drop (this will override AI filter)
				}
			}
		}
		return 0 // Don't influence
	}

	// Use both AI filtering and custom filter
	err := Enable(tp,
		braintrust.WithDefaultProjectID("combined-filter-test"),
		braintrust.WithFilterAISpans(true),
		braintrust.WithSpanFilterFuncs(customFilter),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// AI span - should be kept by AI filter
	_, span1 := tracer.Start(context.Background(), "gen_ai.completion")
	span1.End()

	// Important span - should be kept by custom filter
	_, span2 := tracer.Start(context.Background(), "critical-operation", trace.WithAttributes(
		attribute.String("importance", "high"),
	))
	span2.End()

	// Neither AI nor important - should be filtered out
	_, span3 := tracer.Start(context.Background(), "routine-operation")
	span3.End()

	// Test priority: custom filter overrides AI filter
	// This span has AI attributes but custom filter should drop it (custom filter has priority)
	_, span4 := tracer.Start(context.Background(), "gen_ai.bad_completion", trace.WithAttributes(
		attribute.String("gen_ai.request.model", "gpt-4"),
		attribute.String("importance", "low"), // Custom filter will return 0, AI filter would return 1, but custom comes first
	))
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Should have both AI span and important span, but not the routine or bad completion
	assert.Len(spans, 2)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "gen_ai.completion")
	assert.Contains(spanNames, "critical-operation")
	assert.NotContains(spanNames, "routine-operation")
	assert.NotContains(spanNames, "gen_ai.bad_completion") // AI span but custom filter had priority
}
