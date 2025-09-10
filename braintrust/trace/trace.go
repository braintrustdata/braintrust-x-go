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

// Enable configures adds Braintrust tracing to an existing OpenTelemetry tracer provider.
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

	// split url and protocol
	parts := strings.Split(url, "://")
	if len(parts) != 2 {
		return fmt.Errorf("invalid url: %s", url)
	}
	protocol := parts[0]
	url = parts[1]

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

	// Use provided exporter or create Braintrust OTLP exporter
	exporter := config.SpanExporter
	if exporter == nil {
		var err error
		exporter, err = otlptrace.New(
			context.Background(),
			otlptracehttp.NewClient(otelOpts...),
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	}

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

	parent := Parent{Type: parentType, ID: parentID}
	spanProcessorOpt := WithDefaultParent(parent)

	// FIXME[matt] the NewSpanProcessor is only registered for the effect of mutating spans.
	tp.RegisterSpanProcessor(trace.NewBatchSpanProcessor(exporter))
	tp.RegisterSpanProcessor(NewSpanProcessor(spanProcessorOpt))

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

// SetParent will set the parent to the given Parent for any span created from the returned context.
// Example:
//
//	projectID := "123-456-789"
//	project := trace.NewProject(projectID)
//	ctx = trace.SetParent(ctx, project)
//
//	// All spans created from this context will be assigned to project 123-456-789
//	_, span := tracer.Start(ctx, "database-query")
//	defer span.End()
func SetParent(ctx context.Context, parent Parent) context.Context {
	return context.WithValue(ctx, parentContextKey, parent)
}

// GetParent returns the parent from the context and a boolean indicating if it was set.
func GetParent(ctx context.Context) (bool, Parent) {
	parent, ok := ctx.Value(parentContextKey).(Parent)
	return ok, parent
}

// ParentType is the type of parent.
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

func (p Parent) valid() bool {
	return p.Type != "" && p.ID != ""
}

func (p Parent) String() string {
	return fmt.Sprintf("%s:%s", p.Type, p.ID)
}

// SpanProcessor is an OTel span processor that labels spans with their parent key.
// It must be included in the OTel pipeline to send data to Braintrust.
type SpanProcessor interface {
	trace.SpanProcessor
}

type spanProcessor struct {
	defaultParent Parent
	defaultAttr   attribute.KeyValue
}

// SpanProcessorOption configures the span processor.
type SpanProcessorOption func(*spanProcessor)

// WithDefaultParent sets the default parent for all spans that don't explicitly have one.
func WithDefaultParent(parent Parent) SpanProcessorOption {
	log.Debugf("Setting default parent: %s:%s", parent.Type, parent.ID)
	return func(p *spanProcessor) {
		p.defaultParent = parent
	}
}

// NewSpanProcessor creates a new span processor. All spans must be tagged with a parent (e.g. an experiment_id or project_id).
func NewSpanProcessor(opts ...SpanProcessorOption) SpanProcessor {
	p := &spanProcessor{}
	for _, opt := range opts {
		opt(p)
	}

	if p.defaultParent.valid() {
		p.defaultAttr = attribute.String(ParentOtelAttrKey, p.defaultParent.String())
	}

	return p
}

// OnStart is called when a span is started and assigns parent attributes.
// It assigns spans to projects or experiments based on context or default parent.
func (p *spanProcessor) OnStart(ctx context.Context, span trace.ReadWriteSpan) {
	// If that span already has a parent, don't override
	for _, attr := range span.Attributes() {
		if attr.Key == ParentOtelAttrKey && attr.Value.AsString() != "" {
			log.Debugf("SpanProcessor.OnStart: noop. Span has parent %s", attr.Value.AsString())
			return
		}
	}

	// if the context has a parent, use it.
	ok, parent := GetParent(ctx)
	if ok {
		setParentOnSpan(span, parent)
		log.Debugf("SpanProcessor.OnStart: setting parent from context: %s", parent)
		return
	}

	// otherwise use the default parent
	if p.defaultParent.valid() {
		span.SetAttributes(p.defaultAttr)
		log.Debugf("SpanProcessor.OnStart: setting default parent: %s", p.defaultParent)
	}
}

// OnEnd is called when a span ends.
func (*spanProcessor) OnEnd(_ trace.ReadOnlySpan) {}

// Shutdown shuts down the span processor.
func (*spanProcessor) Shutdown(_ context.Context) error { return nil }

// ForceFlush forces a flush of the span processor.
func (*spanProcessor) ForceFlush(_ context.Context) error { return nil }

var _ trace.SpanProcessor = &spanProcessor{}

func setParentOnSpan(span trace.ReadWriteSpan, parent Parent) {
	span.SetAttributes(attribute.String(ParentOtelAttrKey, parent.String()))
}
