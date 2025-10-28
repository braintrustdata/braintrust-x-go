// Package trace provides distributed tracing functionality for Braintrust experiments.
//
// This package is built on OpenTelemetry and integrates with the Braintrust Client
// for session-based authentication.
//
// To enable tracing, create a Braintrust client (uses NewWithOtel for automatic setup):
//
//	bt, err := braintrust.NewWithOtel(
//	    braintrust.WithAPIKey(os.Getenv("BRAINTRUST_API_KEY")),
//	    braintrust.WithProject("my-project"),
//	)
//	defer bt.Shutdown(context.Background())
//
// Once the client is created, create spans using OpenTelemetry:
//
//	tracer := otel.Tracer("my-app")
//	ctx, span := tracer.Start(ctx, "my-operation")
//	span.SetAttributes(attribute.String("user.id", "123"))
//	span.End()
package trace

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/logger"
)

// Config holds configuration for Braintrust tracing
type Config struct {
	// Default parent for spans
	DefaultProjectID   string
	DefaultProjectName string

	// Span filtering
	FilterAISpans   bool
	SpanFilterFuncs []SpanFilterFunc

	// Debug
	EnableConsoleLog bool

	// Test override: provide custom exporter (e.g., memory exporter for tests)
	Exporter sdktrace.SpanExporter

	// Logger
	Logger logger.Logger
}

// SpanFilterFunc decides which spans to send to Braintrust.
// Return >0 to keep, <0 to drop, 0 to not influence.
type SpanFilterFunc func(span sdktrace.ReadOnlySpan) int

// GetSpanProcessor creates a Braintrust span processor.
func GetSpanProcessor(session *auth.Session, cfg Config) (sdktrace.SpanProcessor, error) {
	log := cfg.Logger
	if log == nil {
		log = logger.NewDefaultLogger()
	}

	// Get API endpoints - always available immediately
	endpoints := session.Endpoints()

	var exporter sdktrace.SpanExporter
	var err error

	// Use provided exporter (for tests) or create HTTP OTLP exporter
	if cfg.Exporter != nil {
		exporter = cfg.Exporter
		log.Debug("using provided exporter")
	} else {
		otelOpts, err := getHTTPOtelOpts(endpoints.APIURL, endpoints.APIKey)
		if err != nil {
			return nil, err
		}

		exporter, err = otlptrace.New(
			context.Background(),
			otlptracehttp.NewClient(otelOpts...),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		log.Debug("created OTLP HTTP exporter", "endpoint", endpoints.APIURL)
	}

	// Wrap in batch processor
	batchProcessor := sdktrace.NewBatchSpanProcessor(exporter)

	// Get default parent from config
	parent := getParent(cfg)
	log.Debug("using default parent", "parent", parent.String())

	// Build filter list
	var filters []SpanFilterFunc
	filters = append(filters, cfg.SpanFilterFuncs...)
	if cfg.FilterAISpans {
		filters = append(filters, aiSpanFilterFunc)
		log.Debug("AI span filtering enabled")
	}

	// Wrap with Braintrust span processor (adds parent labels, filtering, etc.)
	// The processor will get endpoints and org name from session dynamically
	btProcessor, err := newSpanProcessor(
		batchProcessor,
		parent,
		filters,
		session,
		log,
	)
	if err != nil {
		return nil, err
	}

	return btProcessor, nil
}

// AddSpanProcessor creates and registers a Braintrust span processor.
func AddSpanProcessor(tp *sdktrace.TracerProvider, session *auth.Session, cfg Config) error {
	log := cfg.Logger
	if log == nil {
		log = logger.NewDefaultLogger()
	}

	processor, err := GetSpanProcessor(session, cfg)
	if err != nil {
		return err
	}

	tp.RegisterSpanProcessor(processor)
	log.Debug("registered Braintrust span processor")

	// Add console log processor if requested
	if cfg.EnableConsoleLog {
		consoleExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err == nil {
			tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(consoleExporter))
			log.Debug("registered console trace exporter")
		} else {
			log.Warn("failed to create console exporter", "error", err)
		}
	}

	return nil
}

// ParentOtelAttrKey is the OpenTelemetry attribute key used to associate spans with Braintrust parents.
// This enables spans to be grouped under specific projects or experiments in the Braintrust platform.
// Parents are formatted as "project_id:{uuid}" or "experiment_id:{uuid}".
const ParentOtelAttrKey = "braintrust.parent"

// Internal attribute keys for Braintrust span metadata.
const (
	orgAttrKey    = "braintrust.org"
	appURLAttrKey = "braintrust.app_url"
)

type contextKey string

// a context key that cannot possibly collide with any other keys
var parentContextKey contextKey = ParentOtelAttrKey

// SetParent will add a parent to the given context. Any span started with that context will
// be marked with that parent, and sent to the given project or experiment in Braintrust.
//
// Example:
//
//	ctx = trace.SetParent(ctx, trace.Parent{Type: trace.ParentTypeProjectName, ID: "my-project"})
//	_, span := tracer.Start(ctx, "database-query")
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
	// ParentTypeProjectName is the type of parent that represents a project by name.
	ParentTypeProjectName ParentType = "project_name"
	// ParentTypeProjectID is the type of parent that represents a project by ID.
	ParentTypeProjectID ParentType = "project_id"
	// ParentTypeExperimentID is the type of parent that represents an experiment by ID.
	ParentTypeExperimentID ParentType = "experiment_id"
)

// IsValid returns true if the ParentType is a valid type.
func (p ParentType) IsValid() bool {
	return p == ParentTypeProjectName || p == ParentTypeProjectID || p == ParentTypeExperimentID
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
// applied to spans with no parent, so it's separate.
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
	wrapped   sdktrace.SpanProcessor
	filters   []SpanFilterFunc
	otelAttrs *otelAttrs
	session   *auth.Session // Session provides endpoints and org name
	logger    logger.Logger
}

// newSpanProcessor creates a new span processor that wraps another processor and adds parent labeling.
func newSpanProcessor(
	proc sdktrace.SpanProcessor,
	defaultParent Parent,
	filters []SpanFilterFunc,
	session *auth.Session,
	log logger.Logger,
) (*spanProcessor, error) {
	// Get endpoints from session
	endpoints := session.Endpoints()

	// Initialize with empty org name - will be looked up dynamically from session
	attrs := newOtelAttrs(defaultParent, "", endpoints.AppURL)

	sp := &spanProcessor{
		wrapped:   proc,
		filters:   filters,
		otelAttrs: attrs,
		session:   session,
		logger:    log,
	}

	return sp, nil
}

// OnStart is called when a span is started and assigns parent attributes.
// It assigns spans to projects or experiments based on context or default parent.
func (sp *spanProcessor) OnStart(ctx context.Context, span sdktrace.ReadWriteSpan) {
	// Always check session for latest org name and appURL (non-blocking)
	// If login hasn't completed yet, OrgName() returns empty string
	orgName := sp.session.OrgName()
	if orgName != "" {
		sp.otelAttrs.SetOrgName(orgName)
	}

	// Update appURL in case it changed
	endpoints := sp.session.Endpoints()
	sp.otelAttrs.appURL = endpoints.AppURL

	defaultParent, attrs := sp.otelAttrs.Get()

	// All otel spans need to have a parent attached (e.g. project_name:my-project).
	// If the span doesn't have one already attached, use the one from the context or our default.
	if !hasParent(span) {
		// if the context has a parent, use it.
		ok, parent := GetParent(ctx)
		if ok {
			setParentOnSpan(span, parent)
			sp.logger.Debug("setting parent from context", "parent", parent.String())
		} else {
			// otherwise use the default parent
			span.SetAttributes(defaultParent)
			sp.logger.Debug("setting default parent", "parent", defaultParent.Value.AsString())
		}
	}

	// Set any other additional attributes (org name, app URL, etc.)
	span.SetAttributes(attrs...)

	// Delegate to wrapped processor
	sp.wrapped.OnStart(ctx, span)
}

// OnEnd is called when a span ends.
func (sp *spanProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	// Apply filters to determine if we should forward this span
	if sp.shouldForwardSpan(span) {
		sp.wrapped.OnEnd(span)
	}
}

// shouldForwardSpan applies filter functions to determine if a span should be forwarded.
// Root spans are always kept. Filter functions are applied in order, with the first filters having priority.
func (sp *spanProcessor) shouldForwardSpan(span sdktrace.ReadOnlySpan) bool {
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

var _ sdktrace.SpanProcessor = &spanProcessor{}

func setParentOnSpan(span sdktrace.ReadWriteSpan, parent Parent) {
	span.SetAttributes(parent.Attr())
}

// getParent determines the default parent from the config
func getParent(cfg Config) Parent {
	// Figure out our default parent (defaulting to a reasonable value so users can still
	// see data flowing with no default project set)
	parentType := ParentTypeProjectName
	parentID := "go-otel-default-project"
	switch {
	case cfg.DefaultProjectID != "":
		parentType = ParentTypeProjectID
		parentID = cfg.DefaultProjectID
	case cfg.DefaultProjectName != "":
		parentType = ParentTypeProjectName
		parentID = cfg.DefaultProjectName
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
	urlWithoutProtocol := parts[1]

	otelOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(urlWithoutProtocol),
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

func hasParent(span sdktrace.ReadWriteSpan) bool {
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
func aiSpanFilterFunc(span sdktrace.ReadOnlySpan) int {
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
	readSpan, ok := span.(sdktrace.ReadWriteSpan)
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
