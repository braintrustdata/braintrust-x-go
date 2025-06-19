// Package trace provides distributed tracing functionality for Braintrust experiments.
//
// This package is built on OpenTelemetry and provides an easy way to integrate
// Braintrust tracing into your applications.
//
// For new applications, use Quickstart() to get up and running quickly:
//
//	// First, set your API key: export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
// Once you have the tracer set up, get a tracer instance and create spans:
//
//	tracer := otel.Tracer("my-app")
//	ctx, span := tracer.Start(ctx, "my-operation")
//	span.SetAttributes(attribute.String("user.id", "123"))
//	// ... do work ...
//	span.End()
//
// For existing OpenTelemetry setups, you must add our SpanProcessor to your tracer provider.
//
//	defaultProjectID := "your-project-id"
//	processor := trace.NewSpanProcessor(defaultProjectID)
//	tp := sdktrace.NewTracerProvider(
//		sdktrace.WithSpanProcessor(processor),
//		// ... your other processors
//	)
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

	"github.com/braintrust/braintrust-x-go/braintrust"
	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

// Quickstart configures OpenTelemetry tracing for Braintrust and provides
// an easy way of getting up and running if you are new to OpenTelemetry. It
// returns a teardown function that should be called before your program exits.
//
// Example:
//
//	// Use default project
//	teardown, err := trace.Quickstart()
//
//	// Use specific project
//	teardown, err := trace.Quickstart(trace.WithDefaultProjectID("my-project"))
func Quickstart(opts ...braintrust.Option) (teardown func(), err error) {

	config := braintrust.GetConfig(opts...)
	url := config.APIURL
	apiKey := config.APIKey

	diag.Debugf("Initializing OpenTelemetry tracer with config: %s", config.String())

	// split url and protocol
	parts := strings.Split(url, "://")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid url: %s", url)
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

	// Create Braintrust OTLP exporter
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(otelOpts...),
	)
	if err != nil {
		return nil, err
	}

	// If we have a default project ID, set it on the span processor.
	spanProcessorOpt := noopSpanProcessorOption()
	if config.DefaultProjectID != "" {
		spanProcessorOpt = WithDefaultProjectID(config.DefaultProjectID)
	} else {
		diag.Debugf("No default project ID set. Untagged spans will be dropped")
	}

	tracerOpts := []trace.TracerProviderOption{
		trace.WithBatcher(exporter),
		trace.WithSpanProcessor(NewSpanProcessor(spanProcessorOpt)),
	}

	// Add console debug exporter if BRAINTRUST_ENABLE_TRACE_DEBUG_LOG is set
	if config.EnableTraceDebugLog {
		consoleExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			diag.Warnf("failed to create console exporter: %v", err)
		} else {
			tracerOpts = append(tracerOpts, trace.WithBatcher(consoleExporter))
			diag.Debugf("OTEL console debug enabled")
		}

	}

	// Create a tracer provider with all exporters
	tp := trace.NewTracerProvider(tracerOpts...)
	otel.SetTracerProvider(tp)

	teardown = func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			diag.Warnf("Error shutting down tracer provider: %v", err)
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

// Parent represents where data goes in Braintrust - a project, an experiment, etc.
type Parent interface {
	String() string
}

// Project is a parent that represents a project.
type Project struct {
	id string
}

func (p Project) String() string {
	return fmt.Sprintf("project_id:%s", p.id)
}

var _ Parent = Project{}

// Experiment is a parent that represents an experiment.
type Experiment struct {
	ID string
}

// NewProject creates a new project parent with the given ID.
// The resulting parent will be formatted as "project_id:{id}".
func NewProject(id string) Project {
	return Project{id: id}
}

// NewExperiment creates a new experiment parent with the given ID.
// The resulting parent will be formatted as "experiment_id:{id}".
func NewExperiment(id string) Experiment {
	return Experiment{ID: id}
}

func (e Experiment) String() string {
	return fmt.Sprintf("experiment_id:%s", e.ID)
}

var _ Parent = Experiment{}

// SpanProcessor is an OTel span processor that labels spans with their parent key.
// It must be included in the OTel pipeline to send data to Braintrust.
type SpanProcessor interface {
	trace.SpanProcessor
}

type spanProcessor struct {
	defaultProjectID string
	defaultAttr      attribute.KeyValue
}

// SpanProcessorOption is an option that can be passed to NewSpanProcessor.
type SpanProcessorOption func(*spanProcessor)

func noopSpanProcessorOption() SpanProcessorOption {
	return func(p *spanProcessor) {}
}

// WithDefaultProjectID sets the default project ID for spans created during the session.
func WithDefaultProjectID(projectID string) SpanProcessorOption {
	diag.Debugf("Setting default project ID: %s", projectID)
	return func(p *spanProcessor) {
		p.defaultProjectID = projectID
	}
}

// NewSpanProcessor creates a new span processor. All spans must be tagged with a parent (e.g. an experiment_id or project_id).
func NewSpanProcessor(opts ...SpanProcessorOption) SpanProcessor {
	p := &spanProcessor{}
	for _, opt := range opts {
		opt(p)
	}

	if p.defaultProjectID != "" {
		p.defaultAttr = attribute.String(ParentOtelAttrKey, NewProject(p.defaultProjectID).String())
	}

	return p
}

// OnStart is called when a span is started and assigns parent attributes.
// It assigns spans to projects or experiments based on context or default parent.
func (p *spanProcessor) OnStart(ctx context.Context, span trace.ReadWriteSpan) {
	// If that span already has a parent, don't override
	for _, attr := range span.Attributes() {
		if attr.Key == ParentOtelAttrKey && attr.Value.AsString() != "" {
			diag.Debugf("SpanProcessor.OnStart: noop. Span has parent %s", attr.Value.AsString())
			return
		}
	}

	// if the context has a parent, use it.
	ok, parent := GetParent(ctx)
	if ok {
		setParentOnSpan(span, parent)
		diag.Debugf("SpanProcessor.OnStart: setting parent from context: %s", parent)
		return
	}

	// otherwise use the default parent
	if p.defaultProjectID != "" {
		span.SetAttributes(p.defaultAttr)
		diag.Debugf("SpanProcessor.OnStart: setting default parent: %s", p.defaultProjectID)
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
