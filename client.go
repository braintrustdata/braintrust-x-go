package braintrust

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
	"github.com/braintrustdata/braintrust-x-go/logger"
)

// Client is the main Braintrust SDK client
type Client struct {
	config         *config.Config
	logger         logger.Logger
	session        *auth.Session
	tracerProvider *trace.TracerProvider
	ownedProvider  bool
}

// New creates a new Braintrust client.
//
// Configuration is loaded from environment variables first, then
// explicit options are applied (options take precedence).
//
// By default, New():
//   - Creates and configures an OpenTelemetry TracerProvider
//   - Starts login in the background (async)
//   - Sets the TracerProvider as the global default
//
// Example:
//
//	bt, err := braintrust.New(
//	    braintrust.WithAPIKey("your-api-key"),
//	    braintrust.WithProject("my-project"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer bt.Shutdown(context.Background())
func New(opts ...Option) (*Client, error) {
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
		"tracing_enabled", cfg.TracingEnabled,
		"blocking_login", cfg.BlockingLogin)

	// Create auth session - starts async login immediately
	session, err := auth.NewSession(context.Background(), auth.Options{
		AppURL:       cfg.AppURL,
		AppPublicURL: cfg.AppURL,
		APIKey:       cfg.APIKey,
		OrgName:      cfg.OrgName,
		Logger:       log,
	})
	if err != nil {
		log.Error("failed to create auth session", "error", err)
		return nil, fmt.Errorf("failed to create auth session: %w", err)
	}

	client.session = session

	// Setup tracing (can use session even if login not complete yet)
	if cfg.TracingEnabled {
		if err := client.setupTracing(); err != nil {
			log.Error("failed to setup tracing", "error", err)
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
		log.Debug("tracing setup complete", "owned_provider", client.ownedProvider)
	}

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
	var tp *trace.TracerProvider

	if c.config.TracerProvider != nil {
		// Use provided TracerProvider
		tp = c.config.TracerProvider
		c.tracerProvider = tp
		c.ownedProvider = false
		c.logger.Debug("using provided tracer provider")
	} else {
		// Create new TracerProvider
		tp = trace.NewTracerProvider()
		c.tracerProvider = tp
		c.ownedProvider = true
		c.logger.Debug("created new tracer provider")
	}

	// Enable Braintrust tracing on the provider
	// TODO: Call trace.Enable() once we refactor trace package
	c.logger.Debug("enabling braintrust tracing on provider")

	// Set as global if requested
	if c.config.SetGlobalTracer {
		otel.SetTracerProvider(tp)
		c.logger.Debug("set tracer provider as global")
	}

	return nil
}

// Shutdown gracefully shuts down the client.
// If the client owns the TracerProvider, this flushes and shuts it down.
//
// Always call Shutdown before your program exits:
//
//	defer bt.Shutdown(context.Background())
func (c *Client) Shutdown(ctx context.Context) error {
	c.logger.Debug("shutting down client")

	if c.ownedProvider && c.tracerProvider != nil {
		c.logger.Debug("shutting down tracer provider")
		if err := c.tracerProvider.Shutdown(ctx); err != nil {
			c.logger.Error("error shutting down tracer provider", "error", err)
			return fmt.Errorf("tracer provider shutdown failed: %w", err)
		}
	}

	c.logger.Debug("client shutdown complete")
	return nil
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
  App URL: %s
  Tracing: %v
  Global Tracer: %v`,
		orgInfo,
		c.config.DefaultProjectName,
		c.config.APIURL,
		c.config.AppURL,
		c.config.TracingEnabled,
		c.config.SetGlobalTracer,
	)
}
