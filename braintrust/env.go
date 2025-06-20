package braintrust

import (
	"fmt"
	"os"
	"strings"
)

// Option is used to configure the Braintrust GetConfig function.
type Option func(*Config)

// WithDefaultProjectID sets the default project ID for spans created during the session.
func WithDefaultProjectID(projectID string) Option {
	return func(c *Config) {
		c.DefaultProjectID = projectID
	}
}

// Config holds the configuration for Braintrust
type Config struct {
	APIKey              string
	APIURL              string
	AppURL              string
	DefaultProjectID    string
	// MANU_COMMENT: Should this be something like "EnableTraceConsoleLog" or
	// something? DebugLog makes it sound like we're configuring the logging
	// level to debug.
	EnableTraceDebugLog bool
}

// String returns a pretty-printed representation of the config with the API key redacted
func (c Config) String() string {
	apiKey := "<not set>"
	if c.APIKey != "" {
		apiKey = c.APIKey[:3] + "........" + c.APIKey[len(c.APIKey)-3:]
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
		c.EnableTraceDebugLog,
	)
}

// GetConfig returns the Braintrust configuration from environment variables
func GetConfig(opts ...Option) Config {
	config := Config{
		APIKey:              getEnvString("BRAINTRUST_API_KEY", ""),
		APIURL:              getEnvString("BRAINTRUST_API_URL", "https://api.braintrust.dev"),
		AppURL:              getEnvString("BRAINTRUST_APP_URL", "https://www.braintrust.dev"),
		DefaultProjectID:    getEnvString("BRAINTRUST_DEFAULT_PROJECT_ID", ""),
		EnableTraceDebugLog: getEnvBool("BRAINTRUST_ENABLE_TRACE_DEBUG_LOG", false),
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
