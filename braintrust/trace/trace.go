// Package trace provides distributed tracing functionality for Braintrust experiments.
//
// This package is built on OpenTelemetry and provides an easy way to integrate
// Braintrust tracing into your applications.
//
// If your application doesn't use OpenTelemetry, use Quickstart() to enable
// tracing:
//
//	// First, set your API key: export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
// For existing OpenTelemetry setups, use Enable() to add Braintrust to your tracer provider:
//
//	tracerProvider := otel.GetTracerProvider()
//	err := trace.Enable(tracerProvider)
//
// Once you have the tracer set up, get a tracer instance and create spans:
//
//	tracer := otel.Tracer("my-app")
//	ctx, span := tracer.Start(ctx, "my-operation")
//	span.SetAttributes(attribute.String("user.id", "123"))
//	span.End()
//
// For automatic instrumentation of external libraries like OpenAI, see the
// traceopenai subpackage for ready-to-use middleware.
package trace

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
)

// Enable adds Braintrust tracing to an existing OpenTelemetry tracer provider.
//
// Example:
//
//	// Get the configured global OTel TracerProvider
//	tp := trace.GetTracerProvider()
//
//	// Enable Braintrust tracing on it
//	err := trace.Enable(tp)
//	if err != nil {
//		log.Fatal(err)
//	}
func Enable(tp *trace.TracerProvider, opts ...braintrust.Option) error {
	config := braintrust.GetConfig(opts...)
	url := config.APIURL
	apiKey := config.APIKey

	log.Debugf("Enabling Braintrust tracing with config: %s", config.String())

	processor := config.SpanProcessor
	if processor == nil {
		otelOpts, err := getHTTPOtelOpts(url, apiKey)
		if err != nil {
			return err
		}

		// By default, we use an otlp exporter in batch mode.
		exporter, err := otlptrace.New(
			context.Background(),
			otlptracehttp.NewClient(otelOpts...),
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		processor = trace.NewBatchSpanProcessor(exporter)
	}

	// Figure out our default parent from the config.
	parent := getParent(config)

	// Build filter functions list - user filters first, then automatic AI filter
	var filters []braintrust.SpanFilterFunc
	filters = append(filters, config.SpanFilterFuncs...)
	if config.FilterAISpans {
		filters = append(filters, aiSpanFilterFunc)
	}

	// Wrap the raw OTEL span processor with the bt span processor (which labels the parents,
	// filters data, etc)
	tp.RegisterSpanProcessor(newSpanProcessor(processor, parent, filters))

	// Add console debug exporter if BRAINTRUST_ENABLE_TRACE_DEBUG_LOG is set
	if config.EnableTraceConsoleLog {
		consoleExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			log.Warnf("failed to create console exporter: %v", err)
		} else {
			tp.RegisterSpanProcessor(trace.NewBatchSpanProcessor(consoleExporter))
			log.Debugf("OTEL console debug enabled")
		}
	}

	return nil
}

// Quickstart configures OpenTelemetry tracing and returns a teardown function that should
// be called before your program exits.
//
// Quickstart is an easy way of getting up and running if you are new to OpenTelemetry. Use
// `Enable` instead if you are integrating Braintrust into an application that
// already uses OpenTelemetry.
//
// Example:
//
//	teardown, err := trace.Quickstart()
//	if err != nil {
//			log.Fatal(err)
//	}
//	defer teardown()
func Quickstart(opts ...braintrust.Option) (teardown func(), err error) {
	// Create a tracer provider
	tp := trace.NewTracerProvider()

	// Enable Braintrust tracing on it
	err = Enable(tp, opts...)
	if err != nil {
		return nil, err
	}

	// Set it as the global tracer provider
	otel.SetTracerProvider(tp)

	// Return teardown function that shuts down the tracer provider we created
	teardown = func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			log.Warnf("Error shutting down tracer provider: %v", err)
		}
	}

	return teardown, nil
}

// ParentOtelAttrKey is the OpenTelemetry attribute key used to associate spans with Braintrust parents.
// This enables spans to be grouped under specific projects or experiments in the Braintrust platform.
// Parents are formatted as "project_id:{uuid}" or "experiment_id:{uuid}".
const ParentOtelAttrKey = "braintrust.parent"

type contextKey string

// a context key that cannot possibly collide with any other keys
var parentContextKey contextKey = ParentOtelAttrKey

// SetParent will add a parent to the given context. Any span started with that context will
// be marked with that parent, and sent to the given project or experiment in Braintrust.
//
// Example:
//
//	ctx = trace.SetParent(ctx, trace.Parent{Type: "project_name", ID: "test"})
//	span := tracer.Start(ctx, "database-query")
func SetParent(ctx context.Context, parent Parent) context.Context {
	return context.WithValue(ctx, parentContextKey, parent)
}

// GetParent returns the parent from the context and a boolean indicating if it was set.
func GetParent(ctx context.Context) (bool, Parent) {
	parent, ok := ctx.Value(parentContextKey).(Parent)
	return ok, parent
}

// ParentType represents the different places spans can be sent to
// in Braintrust - projects, experiments, etc.
type ParentType string

const (
	// ParentTypeProject is the type of parent that represents a project.
	ParentTypeProject ParentType = "project_name"
	// ParentTypeProjectID is the type of parent that represents a project ID.
	ParentTypeProjectID ParentType = "project_id"
	// ParentTypeExperimentID is the type of parent that represents an experiment ID.
	ParentTypeExperimentID ParentType = "experiment_id"
)

// Parent represents where data goes in Braintrust - a project, an experiment, etc.
type Parent struct {
	Type ParentType
	ID   string
}

// Attr returns the OTel attribute for this parent.
func (p Parent) Attr() attribute.KeyValue {
	return attribute.String(ParentOtelAttrKey, p.String())
}

func (p Parent) String() string {
	return fmt.Sprintf("%s:%s", p.Type, p.ID)
}

type spanProcessor struct {
	wrapped       trace.SpanProcessor
	defaultParent Parent
	defaultAttr   attribute.KeyValue
	filters       []braintrust.SpanFilterFunc
}

// newSpanProcessor creates a new span processor that wraps another processor and adds parent labeling.
func newSpanProcessor(proc trace.SpanProcessor, defaultParent Parent, filters []braintrust.SpanFilterFunc) *spanProcessor {
	log.Debugf("Creating span processor with default parent: %s:%s", defaultParent.Type, defaultParent.ID)
	return &spanProcessor{
		wrapped:       proc,
		defaultParent: defaultParent,
		defaultAttr:   defaultParent.Attr(),
		filters:       filters,
	}
}

// OnStart is called when a span is started and assigns parent attributes.
// It assigns spans to projects or experiments based on context or default parent.
func (p *spanProcessor) OnStart(ctx context.Context, span trace.ReadWriteSpan) {
	// If that span already has a parent, don't override
	if !hasParent(span) {
		// if the context has a parent, use it.
		ok, parent := GetParent(ctx)
		if ok {
			setParentOnSpan(span, parent)
			log.Debugf("SpanProcessor.OnStart: setting parent from context: %s", parent)
		} else {
			// otherwise use the default parent
			span.SetAttributes(p.defaultAttr)
			log.Debugf("SpanProcessor.OnStart: setting default parent: %s", p.defaultParent)
		}
	}

	// Delegate to wrapped processor
	p.wrapped.OnStart(ctx, span)
}

// OnEnd is called when a span ends.
func (p *spanProcessor) OnEnd(span trace.ReadOnlySpan) {
	// Apply filters to determine if we should forward this span
	if p.shouldForwardSpan(span) {
		p.wrapped.OnEnd(span)
	}
}

// shouldForwardSpan applies filter functions to determine if a span should be forwarded.
// Root spans are always kept. Filter functions are applied in order, with the first filters having priority.
func (p *spanProcessor) shouldForwardSpan(span trace.ReadOnlySpan) bool {
	// Always keep root spans (spans with no parent)
	if !span.Parent().IsValid() {
		return true
	}

	// If no filters, keep everything
	if len(p.filters) == 0 {
		return true
	}

	// Apply filter functions in order - first filter wins
	for _, filter := range p.filters {
		result := filter(span)
		switch {
		case result > 0:
			return true
		case result < 0:
			return false
		case result == 0:
			// No influence, continue to next filter
			continue
		}
	}

	// All filters returned 0 (no influence), default to keep
	return true
}

// Shutdown shuts down the span processor.
func (p *spanProcessor) Shutdown(ctx context.Context) error {
	return p.wrapped.Shutdown(ctx)
}

// ForceFlush forces a flush of the span processor.
func (p *spanProcessor) ForceFlush(ctx context.Context) error {
	return p.wrapped.ForceFlush(ctx)
}

var _ trace.SpanProcessor = &spanProcessor{}

func setParentOnSpan(span trace.ReadWriteSpan, parent Parent) {
	span.SetAttributes(parent.Attr())
}

// getParent determines the default parent from the config
func getParent(config braintrust.Config) Parent {
	// Figure out our default parent (defaulting to some random thing so users can still
	// see data flowing with no default project set)
	parentType := ParentTypeProject
	parentID := "go-otel-default-project"
	switch {
	case config.DefaultProjectID != "":
		parentType = ParentTypeProjectID
		parentID = config.DefaultProjectID
	case config.DefaultProjectName != "":
		parentType = ParentTypeProject
		parentID = config.DefaultProjectName
	}

	return Parent{Type: parentType, ID: parentID}
}

// getHTTPOtelOpts parses the URL and creates OTLP HTTP options with proper security settings
func getHTTPOtelOpts(fullURL, apiKey string) ([]otlptracehttp.Option, error) {
	// split url and protocol
	parts := strings.Split(fullURL, "://")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid url: %s", fullURL)
	}
	protocol := parts[0]
	url := parts[1]

	otelOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(url),
		otlptracehttp.WithURLPath("/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + apiKey,
		}),
	}

	if protocol == "http" {
		otelOpts = append(otelOpts, otlptracehttp.WithInsecure())
	}

	return otelOpts, nil
}

func hasParent(span trace.ReadWriteSpan) bool {
	for _, attr := range span.Attributes() {
		if attr.Key == ParentOtelAttrKey {
			return true
		}
	}
	return false
}

var aiOtelPrefixes = []string{
	"gen_ai.",
	"braintrust.",
	"llm.",
	"ai.",
	"traceloop.",
}

// aiSpanFilterFunc is a SpanFilterFunc that keeps AI spans, drops non-AI spans.
// Root spans are always kept by the core filtering logic.
func aiSpanFilterFunc(span trace.ReadOnlySpan) int {
	// Check span name for AI prefixes
	spanName := span.Name()
	for _, prefix := range aiOtelPrefixes {
		if strings.HasPrefix(spanName, prefix) {
			return 1 // Keep AI spans
		}
	}

	// Check attributes for AI prefixes (exclude the braintrust.parent attribute we automatically add)
	for _, attr := range span.Attributes() {
		attrKey := string(attr.Key)
		// Skip the braintrust.parent attribute that we automatically add to all spans
		if attrKey == ParentOtelAttrKey {
			continue
		}
		for _, prefix := range aiOtelPrefixes {
			if strings.HasPrefix(attrKey, prefix) {
				return 1 // Keep AI spans
			}
		}
	}

	// Drop non-AI spans
	return -1
}
