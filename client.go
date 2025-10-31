package braintrust

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/eval"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/logger"
	bttrace "github.com/braintrustdata/braintrust-x-go/trace"
)

// Client is the main Braintrust SDK client
type Client struct {
	config         *config.Config
	logger         logger.Logger
	session        *auth.Session
	tracerProvider *trace.TracerProvider
}

// New creates a new Braintrust client with the provided TracerProvider.
//
// The TracerProvider is required and should be managed by the caller.
// The client will NOT shut down the provider - you must do this yourself.
//
// Configuration is loaded from environment variables first, then
// explicit options are applied (options take precedence).
//
// Login happens asynchronously in the background by default.
//
// Example:
//
//	tp := trace.NewTracerProvider()
//	bt, err := braintrust.New(tp,
//	    braintrust.WithAPIKey("your-api-key"),
//	    braintrust.WithProject("my-project"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
func New(tp *trace.TracerProvider, opts ...Option) (*Client, error) {
	// Build config from environment variables
	cfg := config.FromEnv()

	// Apply user options (override env vars)
	for _, opt := range opts {
		opt(cfg)
	}

	// Setup default logger if none provided
	log := cfg.Logger
	if log == nil {
		log = logger.NewDefaultLogger()
	}

	client := &Client{
		config: cfg,
		logger: log,
	}

	log.Debug("initializing braintrust client",
		"project", cfg.DefaultProjectName,
		"org", cfg.OrgName,
		"api_url", cfg.APIURL,
		"blocking_login", cfg.BlockingLogin)

	// Create auth session - starts async login immediately
	session, err := auth.NewSession(context.Background(), auth.Options{
		AppURL:       cfg.AppURL,
		AppPublicURL: cfg.AppURL,
		APIURL:       cfg.APIURL,
		APIKey:       cfg.APIKey,
		OrgName:      cfg.OrgName,
		Logger:       log,
	})
	if err != nil {
		log.Error("failed to create auth session", "error", err)
		return nil, fmt.Errorf("failed to create auth session: %w", err)
	}

	client.session = session
	client.tracerProvider = tp

	// Setup tracing with provided TracerProvider
	if err := client.setupTracing(); err != nil {
		log.Error("failed to setup tracing", "error", err)
		return nil, fmt.Errorf("failed to setup tracing: %w", err)
	}
	log.Debug("tracing setup complete")

	// If blocking login requested, wait for it
	if cfg.BlockingLogin {
		log.Debug("waiting for login to complete")
		_, err := session.Login(context.Background())
		if err != nil {
			log.Error("blocking login failed", "error", err)
			return nil, fmt.Errorf("login failed: %w", err)
		}
		log.Debug("blocking login complete")
	}

	return client, nil
}

// setupTracing initializes OpenTelemetry tracing
func (c *Client) setupTracing() error {
	// Build trace config from client config
	traceConfig := bttrace.Config{
		DefaultProjectID:   c.config.DefaultProjectID,
		DefaultProjectName: c.config.DefaultProjectName,
		FilterAISpans:      c.config.FilterAISpans,
		SpanFilterFuncs:    convertSpanFilters(c.config.SpanFilterFuncs),
		EnableConsoleLog:   false,
		Exporter:           c.config.Exporter,
		Logger:             c.logger,
	}

	// Add Braintrust span processor to the provided TracerProvider
	c.logger.Debug("enabling braintrust tracing on provider")
	if err := bttrace.AddSpanProcessor(c.tracerProvider, c.session, traceConfig); err != nil {
		c.logger.Error("failed to setup tracing", "error", err)
		return fmt.Errorf("failed to setup tracing: %w", err)
	}

	return nil
}

// convertSpanFilters converts config.SpanFilterFunc to trace.SpanFilterFunc
func convertSpanFilters(funcs []config.SpanFilterFunc) []bttrace.SpanFilterFunc {
	result := make([]bttrace.SpanFilterFunc, len(funcs))
	for i, f := range funcs {
		result[i] = bttrace.SpanFilterFunc(f)
	}
	return result
}

// String returns a string representation of the client
func (c *Client) String() string {
	// Get org name from auth session if available
	orgName := c.config.OrgName
	orgID := ""
	if ok, info := c.session.Info(); ok {
		orgName = info.OrgName
		orgID = info.OrgID
	}

	orgInfo := orgName
	if orgID != "" {
		orgInfo = fmt.Sprintf("%s (ID: %s)", orgName, orgID)
	} else if orgName == "" {
		orgInfo = "<not logged in>"
	}

	return fmt.Sprintf(`Braintrust Client:
  Organization: %s
  Project: %s
  API URL: %s
  App URL: %s`,
		orgInfo,
		c.config.DefaultProjectName,
		c.config.APIURL,
		c.config.AppURL,
	)
}

// TracerProvider returns the OpenTelemetry TracerProvider used by this client.
// This can be used to create tracers or access the provider for advanced use cases.
func (c *Client) TracerProvider() *trace.TracerProvider {
	return c.tracerProvider
}

// Tracer returns an OpenTelemetry Tracer with the given name.
// This is a convenience method equivalent to calling TracerProvider().Tracer(name, opts...).
//
// Example:
//
//	tracer := client.Tracer("my-app")
//	ctx, span := tracer.Start(ctx, "my-operation")
//	defer span.End()
func (c *Client) Tracer(name string, opts ...oteltrace.TracerOption) oteltrace.Tracer {
	return c.tracerProvider.Tracer(name, opts...)
}

// RunEval runs a one-off evaluation using the given client.
//
// For running multiple evaluations with the same input/output types, consider using
// NewEvaluator to create a reusable evaluator.
//
// Example:
//
//	client, _ := braintrust.New(tp, braintrust.WithProject("my-project"))
//
//	result, err := braintrust.RunEval(ctx, client, eval.Opts[string, string]{
//	    Experiment: "greeting-test",
//	    Cases: eval.NewCases([]eval.Case[string, string]{
//	        {Input: "World", Expected: "Hello, World!"},
//	    }),
//	    Task: func(ctx context.Context, input string) (string, error) {
//	        return "Hello, " + input + "!", nil
//	    },
//	    Scorers: []eval.Scorer[string, string]{
//	        eval.ExactMatch[string, string](),
//	    },
//	})
func RunEval[I, R any](ctx context.Context, client *Client, opts eval.Opts[I, R]) (*eval.Result, error) {
	return eval.Run(ctx, opts, client.config, client.session, client.tracerProvider)
}

// NewEvaluator creates a new evaluator for running multiple evaluations with the same
// input and output types.
//
// Example:
//
//	client, _ := braintrust.New(tp, braintrust.WithProject("my-project"))
//
//	// Create an evaluator for string → string evaluations
//	evaluator := braintrust.NewEvaluator[string, string](client)
//
//	// Run multiple evaluations
//	result1, _ := evaluator.Run(ctx, eval.Opts[string, string]{
//	    Experiment: "test-1",
//	    Cases:      cases1,
//	    Task:       task1,
//	    Scorers:    scorers,
//	})
//
//	result2, _ := evaluator.Run(ctx, eval.Opts[string, string]{
//	    Experiment: "test-2",
//	    Cases:      cases2,
//	    Task:       task2,
//	    Scorers:    scorers,
//	})
func NewEvaluator[I, R any](client *Client) *eval.Evaluator[I, R] {
	return eval.NewEvaluator[I, R](client.session, client.config, client.tracerProvider)
}
