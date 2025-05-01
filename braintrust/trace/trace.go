package trace

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

// Quickstart will configure the OpenTelemetry tracer to
// an easy way of getting up and running if you are new to OpenTelemetry. It
// returns a teardown function that should be called before your program exits.
func Quickstart() (teardown func(), err error) {

	diag.Debugf("Initializing OpenTelemetry tracer with experiment_id: %s", os.Getenv("BRAINTRUST_EXPERIMENT_ID"))

	// Create Braintrust OTLP exporter
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint("api.braintrust.dev"),
			otlptracehttp.WithURLPath("/otel/v1/traces"),
			otlptracehttp.WithHeaders(map[string]string{
				"Authorization": "Bearer " + os.Getenv("BRAINTRUST_API_KEY"),
				"x-bt-parent":   "experiment_id:" + os.Getenv("BRAINTRUST_EXPERIMENT_ID"),
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	// FIXME[matt] this should be a parameter
	defaultParent := Experiment{id: "MATT_FAKE_EXPERIMENT_ID"}

	// Create a tracer provider with both exporters
	tp := trace.NewTracerProvider(
		trace.WithSpanProcessor(NewSpanProcessor(defaultParent)),
		trace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)

	teardown = func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			diag.Warnf("Error shutting down tracer provider: %v", err)
		}
	}

	return teardown, nil
}

const PARENT_ATTR = "x-bt-parent"

type contextKey string

// a context key that cannot possibly collide with any other keys
var parentContextKey contextKey = PARENT_ATTR

// SetParent will set the parent to the given Parent for any span created from the returned context.
func SetParent(ctx context.Context, parent Parent) context.Context {
	return context.WithValue(ctx, parentContextKey, parent)
}

func SetParentOnSpan(span trace.ReadWriteSpan, parent Parent) {
	span.SetAttributes(attribute.String(PARENT_ATTR, parent.String()))
}

func getParent(ctx context.Context) (bool, Parent) {
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
	return fmt.Sprintf("project: %s", p.id)
}

var _ Parent = Project{}

// Experiment is a parent that represents an experiment.
type Experiment struct {
	id string
}

func (e Experiment) String() string {
	return fmt.Sprintf("experiment: %s", e.id)
}

var _ Parent = Experiment{}

// SpanProcessor is an OTel span processor that labels spans with their parent key....
//
//	It must be included in the OTel pipeline to send data to Braintrust.
type SpanProcessor struct {
	defaultParent Parent
	defaultAttr   attribute.KeyValue
}

// NewSpanProcessor creates a new span processor that will assign any unlabelled spans to the default parent.
func NewSpanProcessor(defaultParent Parent) *SpanProcessor {
	// FIXME[matt]: option to drop unlabelled spans?
	return &SpanProcessor{
		defaultParent: defaultParent,
		defaultAttr:   attribute.String(PARENT_ATTR, defaultParent.String()),
	}
}

func (p *SpanProcessor) OnStart(ctx context.Context, span trace.ReadWriteSpan) {
	// If that span already has a parent, don't override
	for _, attr := range span.Attributes() {
		if attr.Key == PARENT_ATTR && attr.Value.AsString() != "" {
			diag.Debugf("SpanProcessor.OnStart: noop. Span has parent %s", attr.Value.AsString())
			return
		}
	}

	ok, parent := getParent(ctx)
	if ok {
		SetParentOnSpan(span, parent)
		// if the context has a parent, use it.
		diag.Debugf("SpanProcessor.OnStart: setting parent from context: %s", parent)
		return
	}

	// otherwise use the default parent
	span.SetAttributes(p.defaultAttr)
	diag.Debugf("SpanProcessor.OnStart: setting default parent: %s", p.defaultParent)
}

func (*SpanProcessor) OnEnd(span trace.ReadOnlySpan)        {}
func (*SpanProcessor) Shutdown(ctx context.Context) error   { return nil }
func (*SpanProcessor) ForceFlush(ctx context.Context) error { return nil }

var _ trace.SpanProcessor = &SpanProcessor{}
