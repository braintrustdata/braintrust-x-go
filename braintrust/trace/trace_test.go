package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

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

	_, span1 := tracer.Start(context.Background(), "test")
	span1.End()
	_ = tp.ForceFlush(context.Background())
	span := flushOne(t, exporter)

	assert.Equal(span.Name, "test")
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:12345")

	ctx := context.Background()
	ctx = SetParent(ctx, newProjectIDParent("67890"))
	_, span2 := tracer.Start(ctx, "test")
	span2.End()
	_ = tp.ForceFlush(context.Background())
	span = flushOne(t, exporter)
	assertAttrEquals(t, span, ParentOtelAttrKey, "project_id:67890")

	// Existing parent attributes should not be overridden
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

	err := Enable(tp,
		braintrust.WithDefaultProject(""),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

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

	tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(m1))

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

	customFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "importance" && attr.Value.AsString() == "high" {
				return 1
			}
			if string(attr.Key) == "noise" && attr.Value.AsBool() {
				return -1
			}
		}
		return 0
	}

	err := Enable(tp,
		braintrust.WithDefaultProjectID("filter-test"),
		braintrust.WithSpanFilterFuncs(customFilter),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "important-operation", trace.WithAttributes(
		attribute.String("importance", "high"),
	))
	span1.End()

	_, span2 := tracer.Start(rootCtx, "noisy-operation", trace.WithAttributes(
		attribute.Bool("noise", true),
	))
	span2.End()

	_, span3 := tracer.Start(rootCtx, "normal-operation")
	span3.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 3)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "root-operation")
	assert.Contains(spanNames, "important-operation")
	assert.Contains(spanNames, "normal-operation")
	assert.NotContains(spanNames, "noisy-operation")
}

func TestSpanProcessor_FilteringWithMultipleProcessors(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()

	allSpansExporter := tracetest.NewInMemoryExporter()
	allSpansProcessor := sdktrace.NewSimpleSpanProcessor(allSpansExporter)
	tp.RegisterSpanProcessor(allSpansProcessor)

	braintrustExporter := tracetest.NewInMemoryExporter()
	braintrustProcessor := sdktrace.NewSimpleSpanProcessor(braintrustExporter)

	dropNoisyFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "noise" && attr.Value.AsBool() {
				return -1
			}
		}
		return 0
	}

	err := Enable(tp,
		braintrust.WithDefaultProjectID("multi-processor-test"),
		braintrust.WithSpanFilterFuncs(dropNoisyFilter),
		withSpanProcessor(braintrustProcessor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "important-operation")
	span1.End()

	_, span2 := tracer.Start(rootCtx, "noisy-operation", trace.WithAttributes(
		attribute.Bool("noise", true),
	))
	span2.End()

	_, span3 := tracer.Start(rootCtx, "normal-operation")
	span3.End()

	_ = tp.ForceFlush(context.Background())

	allSpans := allSpansExporter.GetSpans()
	assert.Len(allSpans, 4)

	allSpanNames := make([]string, len(allSpans))
	for i, span := range allSpans {
		allSpanNames[i] = span.Name
	}
	assert.Contains(allSpanNames, "root-operation")
	assert.Contains(allSpanNames, "important-operation")
	assert.Contains(allSpanNames, "noisy-operation")
	assert.Contains(allSpanNames, "normal-operation")

	braintrustSpans := braintrustExporter.GetSpans()
	assert.Len(braintrustSpans, 3)

	braintrustSpanNames := make([]string, len(braintrustSpans))
	for i, span := range braintrustSpans {
		braintrustSpanNames[i] = span.Name
	}
	assert.Contains(braintrustSpanNames, "root-operation")
	assert.Contains(braintrustSpanNames, "important-operation")
	assert.Contains(braintrustSpanNames, "normal-operation")
	assert.NotContains(braintrustSpanNames, "noisy-operation")
}

func TestAISpanFilterFunc_WithAIPrefixes(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("ai-filter-test"),
		braintrust.WithSpanFilterFuncs(aiSpanFilterFunc),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "openai-call", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("gen_ai.request.model", "gpt-4"),
	))
	span1.End()

	_, span2 := tracer.Start(rootCtx, "braintrust-log", trace.WithAttributes(
		attribute.String("braintrust.log.id", "12345"),
	))
	span2.End()

	_, span3 := tracer.Start(rootCtx, "llm-call", trace.WithAttributes(
		attribute.String("llm.request.type", "completion"),
	))
	span3.End()

	_, span4 := tracer.Start(rootCtx, "ai-service", trace.WithAttributes(
		attribute.String("ai.model.name", "claude"),
	))
	span4.End()

	_, span5 := tracer.Start(rootCtx, "traceloop-span", trace.WithAttributes(
		attribute.String("traceloop.span.kind", "llm"),
	))
	span5.End()

	_, span6 := tracer.Start(rootCtx, "database-query", trace.WithAttributes(
		attribute.String("db.statement", "SELECT * FROM users"),
	))
	span6.End()

	_, span7 := tracer.Start(rootCtx, "http-request")
	span7.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 6)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "root-operation")
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

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "gen_ai.completion")
	span1.End()

	_, span2 := tracer.Start(rootCtx, "braintrust.experiment.log")
	span2.End()

	_, span3 := tracer.Start(rootCtx, "llm.chat_completion")
	span3.End()

	_, span4 := tracer.Start(rootCtx, "user.login")
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 4)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "root-operation")
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

	err := Enable(tp,
		braintrust.WithDefaultProjectID("ai-option-test"),
		braintrust.WithFilterAISpans(true),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "gen_ai.completion", trace.WithAttributes(
		attribute.String("gen_ai.request.model", "gpt-4"),
	))
	span1.End()

	_, span2 := tracer.Start(rootCtx, "normal-http-call", trace.WithAttributes(
		attribute.String("llm.provider", "openai"),
	))
	span2.End()

	_, span3 := tracer.Start(rootCtx, "database-query", trace.WithAttributes(
		attribute.String("db.statement", "SELECT * FROM users"),
	))
	span3.End()

	_, span4 := tracer.Start(rootCtx, "http.request")
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 3)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "root-operation")
	assert.Contains(spanNames, "gen_ai.completion")
	assert.Contains(spanNames, "normal-http-call")
	assert.NotContains(spanNames, "database-query")
	assert.NotContains(spanNames, "http.request")
}

func TestWithFilterAISpans_CombinedWithCustomFilters(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	customFilter := func(span sdktrace.ReadOnlySpan) int {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "importance" {
				switch attr.Value.AsString() {
				case "high":
					return 1
				case "low":
					return -1
				}
			}
		}
		return 0
	}

	err := Enable(tp,
		braintrust.WithDefaultProjectID("combined-filter-test"),
		braintrust.WithFilterAISpans(true),
		braintrust.WithSpanFilterFuncs(customFilter),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	rootCtx, rootSpan := tracer.Start(context.Background(), "root-operation")
	rootSpan.End()

	_, span1 := tracer.Start(rootCtx, "gen_ai.completion")
	span1.End()

	_, span2 := tracer.Start(rootCtx, "critical-operation", trace.WithAttributes(
		attribute.String("importance", "high"),
	))
	span2.End()

	_, span3 := tracer.Start(rootCtx, "routine-operation")
	span3.End()

	// Test priority: custom filter overrides AI filter
	// This span has AI attributes but custom filter should drop it (custom filter has priority)
	_, span4 := tracer.Start(rootCtx, "gen_ai.bad_completion", trace.WithAttributes(
		attribute.String("gen_ai.request.model", "gpt-4"),
		attribute.String("importance", "low"),
	))
	span4.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 3)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "root-operation")
	assert.Contains(spanNames, "gen_ai.completion")
	assert.Contains(spanNames, "critical-operation")
	assert.NotContains(spanNames, "routine-operation")
	assert.NotContains(spanNames, "gen_ai.bad_completion")
}

func TestAISpanFilterFunc_KeepsRootSpans(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("root-spans-test"),
		braintrust.WithFilterAISpans(true),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	_, rootSpan1 := tracer.Start(context.Background(), "http-server-request")
	rootSpan1.End()

	_, rootSpan2 := tracer.Start(context.Background(), "gen_ai.root_completion")
	rootSpan2.End()

	rootCtx, rootSpan3 := tracer.Start(context.Background(), "parent-operation")

	_, childAI := tracer.Start(rootCtx, "llm.child_completion")
	childAI.End()

	_, childNonAI := tracer.Start(rootCtx, "database-query")
	childNonAI.End()

	rootSpan3.End()

	_ = tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	assert.Len(spans, 4)

	spanNames := make([]string, len(spans))
	for i, span := range spans {
		spanNames[i] = span.Name
	}

	assert.Contains(spanNames, "http-server-request")
	assert.Contains(spanNames, "gen_ai.root_completion")
	assert.Contains(spanNames, "parent-operation")
	assert.Contains(spanNames, "llm.child_completion")
	assert.NotContains(spanNames, "database-query")
}

func TestPermalink(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithDefaultProjectID("org-test"),
		braintrust.WithOrgName("test-org"),
		braintrust.WithAppURL("https://app.example.com"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-operation")

	// Test Permalink function with live span
	link, err := Permalink(span)
	assert.NoError(err)
	assert.NotEmpty(link)
	assert.Contains(link, "https://app.example.com")
	assert.Contains(link, "test-org")
	assert.Contains(link, "org-test")
	assert.Contains(link, span.SpanContext().SpanID().String())
	assert.Contains(link, span.SpanContext().TraceID().String())

	span.End()

	_ = tp.ForceFlush(context.Background())
	capturedSpan := flushOne(t, exporter)

	assertAttrEquals(t, capturedSpan, "braintrust.org", "test-org")
	assertAttrEquals(t, capturedSpan, "braintrust.app_url", "https://app.example.com")
}

func TestPermalinkWithExperiment(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	err := Enable(tp,
		braintrust.WithOrgName("test-org"),
		braintrust.WithAppURL("https://app.example.com"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")

	// Create a span with an experiment parent (format: project-name/experiment-id)
	ctx := SetParent(context.Background(), Parent{Type: ParentTypeExperimentID, ID: "test-project/exp-123"})
	_, span := tracer.Start(ctx, "test-operation")

	// Test Permalink function with experiment
	link, err := Permalink(span)
	assert.NoError(err)
	assert.NotEmpty(link)
	assert.Contains(link, "https://app.example.com")
	assert.Contains(link, "test-org")
	assert.Contains(link, "/p/test-project/experiments/exp-123")
	assert.Contains(link, span.SpanContext().SpanID().String())
	assert.Contains(link, span.SpanContext().TraceID().String())

	span.End()
}

func TestPermalinkWithNoopSpan(t *testing.T) {
	assert := assert.New(t)

	// Use noop tracer provider to create a noop span
	noopTP := noop.NewTracerProvider()
	noopTracer := noopTP.Tracer("test")
	_, noopSpan := noopTracer.Start(context.Background(), "noop-span")

	// Should return error, not crash
	link, err := Permalink(noopSpan)
	assert.Error(err)
	assert.Empty(link)
	assert.Contains(err.Error(), "does not support attribute reading")

	noopSpan.End()
}

func TestPermalinkMissingAttributes(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Enable without setting OrgName or AppURL
	err := Enable(tp,
		braintrust.WithDefaultProjectID("test-project"),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-operation")

	// Should return error for missing org
	link, err := Permalink(span)
	assert.Error(err)
	assert.Empty(link)
	assert.Contains(err.Error(), "braintrust.org")

	span.End()
}

func TestEnableWithBlockingLogin(t *testing.T) {
	assert := assert.New(t)

	tp := sdktrace.NewTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	processor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Enable with BlockingLogin option - should block and login synchronously
	err := Enable(tp,
		braintrust.WithAPIKey("___TEST_API_KEY___"),
		braintrust.WithDefaultProjectID("test-project"),
		braintrust.WithBlockingLogin(true),
		withSpanProcessor(processor),
	)
	assert.NoError(err)

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-operation")

	// Org name should be set immediately (not via background login)
	// For test API key, org name should be "test-org-name"
	link, err := Permalink(span)
	assert.NoError(err)
	assert.Contains(link, "test-org-name")

	span.End()
}

func TestBlockingLoginDefaultsFalse(t *testing.T) {
	assert := assert.New(t)

	// Test that BlockingLogin defaults to false
	config := braintrust.GetConfig()
	assert.False(config.BlockingLogin)
}

func TestDistributedTracingPropagation(t *testing.T) {
	assert := assert.New(t)

	// This test simulates distributed tracing across a process boundary.
	// It verifies that braintrust.parent propagates from client to server
	// using OpenTelemetry's W3C Trace Context and Baggage propagation.

	// Setup: Create two separate tracer providers (client and server)
	// with their own in-memory exporters to capture spans

	// Configure propagator for distributed tracing (W3C Trace Context + Baggage)
	// This simulates what happens in real distributed systems like HTTP/gRPC
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // Propagates trace ID, span ID
		propagation.Baggage{},      // Propagates baggage (will be used for braintrust.parent)
	)

	// CLIENT SIDE: Create tracer with Braintrust enabled
	clientExporter := tracetest.NewInMemoryExporter()
	clientTP := sdktrace.NewTracerProvider()
	err := Enable(clientTP,
		withSpanProcessor(sdktrace.NewSimpleSpanProcessor(clientExporter)),
		braintrust.WithDefaultProject("test-distributed-project"),
		braintrust.WithAPIKey("test-api-key"),
	)
	assert.NoError(err)

	clientTracer := clientTP.Tracer("test-client")

	// Set a parent (experiment) in the context
	experimentID := "abc123-distributed-test"
	parent := Parent{Type: ParentTypeExperimentID, ID: experimentID}
	clientCtx := context.Background()
	clientCtx = SetParent(clientCtx, parent)

	// Start a span on the client side
	clientCtx, clientSpan := clientTracer.Start(clientCtx, "client-operation")

	// SIMULATE NETWORK: Extract W3C headers (traceparent + baggage)
	// In real distributed systems, these headers are sent over HTTP/gRPC
	headers := make(map[string]string)
	propagator.Inject(clientCtx, &mapCarrier{headers})

	// Finish client span
	clientSpan.End()
	_ = clientTP.ForceFlush(context.Background())

	// Verify client span has the parent attribute
	clientSpans := clientExporter.GetSpans()
	assert.Len(clientSpans, 1)
	assertAttrEquals(t, clientSpans[0], ParentOtelAttrKey, "experiment_id:"+experimentID)

	// SERVER SIDE: Create new tracer provider (separate process)
	serverExporter := tracetest.NewInMemoryExporter()
	serverTP := sdktrace.NewTracerProvider()
	err = Enable(serverTP,
		withSpanProcessor(sdktrace.NewSimpleSpanProcessor(serverExporter)),
		braintrust.WithDefaultProject("test-distributed-project"),
		braintrust.WithAPIKey("test-api-key"),
	)
	assert.NoError(err)

	serverTracer := serverTP.Tracer("test-server")

	// DESERIALIZE: Inject headers into new context
	serverCtx := context.Background()
	serverCtx = propagator.Extract(serverCtx, &mapCarrier{headers})

	// Start a span on the server side with the propagated context
	serverCtx, serverSpan := serverTracer.Start(serverCtx, "server-operation")
	serverSpan.End()
	_ = serverTP.ForceFlush(context.Background())

	// VERIFY: Server span should have the braintrust.parent attribute
	serverSpans := serverExporter.GetSpans()
	assert.Len(serverSpans, 1)

	// The critical assertion: parent should have propagated across the boundary
	assertAttrEquals(t, serverSpans[0], ParentOtelAttrKey, "experiment_id:"+experimentID)

	// Additional verification: trace IDs should match (standard OTel behavior)
	assert.Equal(clientSpans[0].SpanContext.TraceID(), serverSpans[0].SpanContext.TraceID(),
		"Trace IDs should match across distributed boundary")
}

// mapCarrier implements propagation.TextMapCarrier for testing
type mapCarrier struct {
	data map[string]string
}

func (c *mapCarrier) Get(key string) string {
	return c.data[key]
}

func (c *mapCarrier) Set(key, value string) {
	c.data[key] = value
}

func (c *mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}
