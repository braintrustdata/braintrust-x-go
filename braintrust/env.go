package braintrust

import (
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel/sdk/trace"
)

// Option is used to configure the Braintrust GetConfig function.
type Option func(*Config)

// WithDefaultProjectID sets the default project ID for spans created during the session.
func WithDefaultProjectID(projectID string) Option {
	return func(c *Config) {
		c.DefaultProjectID = projectID
	}
}

// WithDefaultProject sets the default project name for spans created during the session.
func WithDefaultProject(projectName string) Option {
	return func(c *Config) {
		c.DefaultProjectName = projectName
	}
}

// WithAPIKey sets the API key for the Braintrust SDK.
func WithAPIKey(apiKey string) Option {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// WithAPIURL sets the API URL for the Braintrust SDK.
func WithAPIURL(apiURL string) Option {
	return func(c *Config) {
		c.APIURL = apiURL
	}
}

// SpanFilterFunc is a function that you can use to decide which spans to send to Braintrust.
// Return >1 to keep the span, <1 to drop the span, or 0 to not influence the decision.
type SpanFilterFunc func(span trace.ReadOnlySpan) int

// WithSpanFilterFuncs sets the SpanFilterFuncs for the Braintrust SDK.
func WithSpanFilterFuncs(filterFuncs ...SpanFilterFunc) Option {
	return func(c *Config) {
		c.SpanFilterFuncs = filterFuncs
	}
}

// WithFilterAISpans enables filtering to keep only AI-related spans.
// When enabled, only spans with AI-related names or attributes will be sent to Braintrust.
// Environment variable: BRAINTRUST_OTEL_FILTER_AI_SPANS
func WithFilterAISpans(enabled bool) Option {
	return func(c *Config) {
		c.FilterAISpans = enabled
	}
}

// Config holds the configuration for the Braintrust SDK
type Config struct {
	APIKey                string
	APIURL                string
	AppURL                string
	DefaultProjectID      string
	DefaultProjectName    string
	EnableTraceConsoleLog bool
	FilterAISpans         bool
	SpanFilterFuncs       []SpanFilterFunc

	// SpanProcessor allows overriding the default SpanProcessor (primarily for testing)
	SpanProcessor trace.SpanProcessor
}

// String returns a pretty-printed representation of the config with the API key redacted
func (c Config) String() string {
	var apiKey string
	if len(c.APIKey) > 6 {
		apiKey = c.APIKey[:3] + "........" + c.APIKey[len(c.APIKey)-3:]
	} else if len(c.APIKey) > 0 {
		apiKey = "<redacted>"
	} else {
		apiKey = "<not set>"
	}

	return fmt.Sprintf(`Braintrust Config:
  APIKey: %s
  APIURL: %s
  AppURL: %s
  DefaultProjectID: %s
  EnableTraceDebugLog: %t`,
		apiKey,
		c.APIURL,
		c.AppURL,
		c.DefaultProjectID,
		c.EnableTraceConsoleLog,
	)
}

// GetConfig loads the Braintrust configuration from environment variables
// and options. Options take precedence over environment variables.
func GetConfig(opts ...Option) Config {
	config := Config{
		APIKey:                getEnvString("BRAINTRUST_API_KEY", ""),
		APIURL:                getEnvString("BRAINTRUST_API_URL", "https://api.braintrust.dev"),
		AppURL:                getEnvString("BRAINTRUST_APP_URL", "https://www.braintrust.dev"),
		DefaultProjectID:      getEnvString("BRAINTRUST_DEFAULT_PROJECT_ID", ""),
		DefaultProjectName:    getEnvString("BRAINTRUST_DEFAULT_PROJECT", "default-go-project"),
		EnableTraceConsoleLog: getEnvBool("BRAINTRUST_ENABLE_TRACE_CONSOLE_LOG", false),
		FilterAISpans:         getEnvBool("BRAINTRUST_OTEL_FILTER_AI_SPANS", false),
	}
	for _, opt := range opts {
		opt(&config)
	}
	return config
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}
