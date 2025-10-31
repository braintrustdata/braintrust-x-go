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
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
)

// Enable adds Braintrust tracing to an existing OpenTelemetry tracer provider.
//
// For distributed tracing across process boundaries (e.g., microservices, Temporal workflows,
// gRPC services), you must also configure OpenTelemetry propagators to propagate the
// braintrust.parent attribute via baggage:
//
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
//		propagation.TraceContext{},
//		propagation.Baggage{},
//	))
//
// See examples/temporal for a complete distributed tracing example.
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
	orgName := config.OrgName

	log.Debugf("Enabling Braintrust tracing with config: %s", config.String())

	// If no orgName set and BlockingLogin enabled, login synchronously
	if orgName == "" && config.BlockingLogin {
		log.Debugf("BlockingLogin enabled, calling Login")
		state, err := auth.Login(auth.Options{
			AppURL:  config.AppURL,
			APIKey:  apiKey,
			OrgName: config.OrgName,
		})
		if err != nil {
			return fmt.Errorf("blocking login failed: %w", err)
		}
		orgName = state.OrgName
		log.Debugf("Login completed, orgName: %q", orgName)
	}
	// If still no orgName, background login will be triggered later by spanProcessor

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
	sp, err := newSpanProcessor(processor, parent, filters, orgName, config.AppURL, apiKey)
	if err != nil {
		return err
	}
	tp.RegisterSpanProcessor(sp)

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
// For distributed tracing across process boundaries (e.g., microservices, Temporal workflows,
// gRPC services), you must also configure OpenTelemetry propagators to propagate the
// braintrust.parent attribute via baggage:
//
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
//		propagation.TraceContext{},
//		propagation.Baggage{},
//	))
//
// See examples/temporal for a complete distributed tracing example.
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

// Internal attribute keys for Braintrust span metadata.
const (
	orgAttrKey     = "braintrust.org"
	appURLAttrKey  = "braintrust.app_url"
	projectAttrKey = "braintrust.project"
)

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
	// Store parent in context value (for same-process access)
	ctx = context.WithValue(ctx, parentContextKey, parent)

	// Also store in baggage for distributed tracing across process boundaries.
	// Baggage propagates automatically through W3C headers, while context values don't.
	member, err := baggage.NewMember(ParentOtelAttrKey, parent.String())
	if err != nil {
		log.Warnf("Failed to create baggage member for parent: %v", err)
		return ctx
	}

	// Merge with existing baggage if any
	existingBag := baggage.FromContext(ctx)
	bag, err := existingBag.SetMember(member)
	if err != nil {
		log.Warnf("Failed to set baggage member for parent: %v", err)
		return ctx
	}

	return baggage.ContextWithBaggage(ctx, bag)
}

// GetParent returns the parent from the context and a boolean indicating if it was set.
// It first checks the context value (for same-process access), then falls back to
// baggage (for distributed tracing across process boundaries).
func GetParent(ctx context.Context) (bool, Parent) {
	// First, try to get from context value (fast path for same-process)
	if parent, ok := ctx.Value(parentContextKey).(Parent); ok {
		return true, parent
	}

	// Fall back to baggage (for distributed tracing)
	bag := baggage.FromContext(ctx)
	if parentStr := bag.Member(ParentOtelAttrKey).Value(); parentStr != "" {
		parent, err := parseParent(parentStr)
		if err != nil {
			log.Warnf("Failed to parse parent from baggage: %v", err)
			return false, Parent{}
		}
		return true, parent
	}

	return false, Parent{}
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

// IsValid returns true if the ParentType is a valid type.
func (p ParentType) IsValid() bool {
	return p == ParentTypeProject || p == ParentTypeProjectID || p == ParentTypeExperimentID
}

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

func parseParent(s string) (Parent, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Parent{}, fmt.Errorf("invalid parent format: %s", s)
	}
	pt := ParentType(parts[0])
	if !pt.IsValid() {
		return Parent{}, fmt.Errorf("invalid parent type: %s", parts[0])
	}

	return Parent{Type: pt, ID: parts[1]}, nil
}

// otelAttrs contains the attributes that are added to all spans in our processor.
type otelAttrs struct {
	parent attribute.KeyValue

	mu sync.RWMutex

	orgName string
	appURL  string

	attrs []attribute.KeyValue
}

func newOtelAttrs(parent Parent, orgName string, appURL string) *otelAttrs {
	oa := &otelAttrs{
		parent:  parent.Attr(),
		orgName: orgName,
		appURL:  appURL,
	}
	oa.makeAttrs()
	return oa
}

// Get returns the attributes that should be set on the span. The parent is selectively
// applied to spans with no parent, there it's separate.
func (o *otelAttrs) Get() (parent attribute.KeyValue, always []attribute.KeyValue) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.parent, o.attrs
}

func (o *otelAttrs) SetOrgName(orgName string) {
	o.mu.Lock()
	o.orgName = orgName
	o.mu.Unlock()

	o.makeAttrs()
}

func (o *otelAttrs) makeAttrs() {
	var attrs []attribute.KeyValue
	if o.orgName != "" {
		attrs = append(attrs, attribute.String(orgAttrKey, o.orgName))
	}
	if o.appURL != "" {
		attrs = append(attrs, attribute.String(appURLAttrKey, o.appURL))
	}

	o.mu.Lock()
	o.attrs = attrs
	o.mu.Unlock()
}

type spanProcessor struct {
	wrapped   trace.SpanProcessor
	filters   []braintrust.SpanFilterFunc
	apiKey    string
	appURL    string
	otelAttrs *otelAttrs
}

// newSpanProcessor creates a new span processor that wraps another processor and adds parent labeling.
func newSpanProcessor(proc trace.SpanProcessor, defaultParent Parent, filters []braintrust.SpanFilterFunc, orgName, appURL, apiKey string) (*spanProcessor, error) {
	if apiKey == "" || appURL == "" {
		return nil, fmt.Errorf("apiKey and appURL are required")
	}

	attrs := newOtelAttrs(defaultParent, orgName, appURL)

	sp := &spanProcessor{
		apiKey:    apiKey,
		appURL:    appURL,
		wrapped:   proc,
		filters:   filters,
		otelAttrs: attrs,
	}

	// If we have no explicit org name, fetch it from the API. This will let us format
	// links like https://www.braintrust.dev/app/<my-org-name>. We add all data needed
	// to create links on spans in advance so that we don't rely on looking up global state
	// later.
	if orgName == "" {
		go sp.login()
	}

	return sp, nil
}

// login will attempt to login until it succeeds so that we can look up the active org name.
func (sp *spanProcessor) login() {
	log.Debugf("spanProcessor: no orgName configured, calling LoginUntilSuccess")
	state, err := auth.LoginUntilSuccess(auth.Options{
		AppURL: sp.appURL,
		APIKey: sp.apiKey,
	})
	if err != nil {
		log.Warnf("spanProcessor: LoginUntilSuccess failed: %v", err)
		return
	}
	log.Debugf("spanProcessor: LoginUntilSuccess succeeded, setting orgName to %q", state.OrgName)
	if state.OrgName != "" {
		sp.otelAttrs.SetOrgName(state.OrgName)
	}
}

// OnStart is called when a span is started and assigns parent attributes.
// It assigns spans to projects or experiments based on context or default parent.
func (sp *spanProcessor) OnStart(ctx context.Context, span trace.ReadWriteSpan) {
	defaultParent, attrs := sp.otelAttrs.Get()

	// All otel spans need to have a parent attached (e.g. project-id:12345). If the span
	// doesn't have one already attached, use the one from the context or our default.
	if !hasParent(span) {
		// if the context has a parent, use it.
		ok, parent := GetParent(ctx)
		if ok {
			setParentOnSpan(span, parent)
			log.Debugf("SpanProcessor.OnStart: setting parent from context: %s", parent)
		} else {
			// otherwise use the default parent
			span.SetAttributes(defaultParent)
			log.Debugf("SpanProcessor.OnStart: setting default parent: %s", defaultParent.Value.AsString())
		}
	}

	// Set any other additional attributes (org name, app URL, etc.)
	span.SetAttributes(attrs...)

	// Delegate to wrapped processor
	sp.wrapped.OnStart(ctx, span)
}

// OnEnd is called when a span ends.
func (sp *spanProcessor) OnEnd(span trace.ReadOnlySpan) {
	// Apply filters to determine if we should forward this span
	if sp.shouldForwardSpan(span) {
		sp.wrapped.OnEnd(span)
	}
}

// shouldForwardSpan applies filter functions to determine if a span should be forwarded.
// Root spans are always kept. Filter functions are applied in order, with the first filters having priority.
func (sp *spanProcessor) shouldForwardSpan(span trace.ReadOnlySpan) bool {
	// Always keep root spans (spans with no parent)
	if !span.Parent().IsValid() {
		return true
	}

	// If no filters, keep everything
	if len(sp.filters) == 0 {
		return true
	}

	// Apply filter functions in order - first filter wins
	for _, filter := range sp.filters {
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
func (sp *spanProcessor) Shutdown(ctx context.Context) error {
	return sp.wrapped.Shutdown(ctx)
}

// ForceFlush forces a flush of the span processor.
func (sp *spanProcessor) ForceFlush(ctx context.Context) error {
	return sp.wrapped.ForceFlush(ctx)
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

	// Check attributes for AI prefixes (exclude system attributes we automatically add)
	for _, attr := range span.Attributes() {
		attrKey := string(attr.Key)
		// Skip system attributes that we automatically add to all spans
		if attrKey == ParentOtelAttrKey || attrKey == orgAttrKey || attrKey == appURLAttrKey {
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

// Permalink returns a URL to the span in the Braintrust UI.
func Permalink(span oteltrace.Span) (string, error) {
	appURL, org, parent, err := getSpanURLData(span)
	if err != nil {
		return "", err
	}

	// Get span context for trace and span IDs
	spanContext := span.SpanContext()
	traceID := spanContext.TraceID().String()
	spanID := spanContext.SpanID().String()

	// Build permalink
	u, err := url.Parse(appURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse app URL: %w", err)
	}

	// Different URL formats based on parent type
	// Projects: {app_url}/app/{org}/p/{project}/logs?r={trace_id}&s={span_id}
	// Experiments: {app_url}/app/{org}/p/{project}/experiments/{experiment_id}?r={trace_id}&s={span_id}
	if parent.Type == ParentTypeExperimentID {
		// For experiments, parent.ID format is "project-name/experiment-id"
		parts := strings.SplitN(parent.ID, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("experiment parent ID must be in format 'project/experiment-id', got: %s", parent.ID)
		}
		projectName := parts[0]
		experimentID := parts[1]
		u = u.JoinPath("app", org, "p", projectName, "experiments", experimentID)
	} else {
		u = u.JoinPath("app", org, "p", parent.ID, "logs")
	}

	q := u.Query()
	q.Set("r", traceID)
	q.Set("s", spanID)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func getSpanURLData(span oteltrace.Span) (url, org string, parent Parent, err error) {
	readSpan, ok := span.(trace.ReadWriteSpan)
	if !ok {
		err = fmt.Errorf("span does not support attribute reading")
		return
	}

	attrs := make(map[string]string)
	for _, attr := range readSpan.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}

	keys := []string{appURLAttrKey, orgAttrKey, ParentOtelAttrKey}
	for _, key := range keys {
		if _, ok := attrs[key]; !ok {
			err = fmt.Errorf("span missing %s attribute", key)
			return
		}
	}

	parent, err = parseParent(attrs[ParentOtelAttrKey])
	if err != nil {
		return
	}

	url = attrs[appURLAttrKey]
	org = attrs[orgAttrKey]
	return
}
