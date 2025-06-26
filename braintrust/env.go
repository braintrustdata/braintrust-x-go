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

// WithAPIKey sets the API key for the Braintrust SDK.
func WithAPIKey(apiKey string) Option {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// Config holds the configuration for the Braintrust SDK
type Config struct {
	APIKey                string
	APIURL                string
	AppURL                string
	DefaultProjectID      string
	EnableTraceConsoleLog bool
}

// String returns a pretty-printed representation of the config with the API key redacted
func (c Config) String() string {
	apiKey := "<not set>"
	if len(c.APIKey) > 6 {
		apiKey = c.APIKey[:3] + "........" + c.APIKey[len(c.APIKey)-3:]
	} else {
		apiKey = "<redacted>"
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
		EnableTraceConsoleLog: getEnvBool("BRAINTRUST_ENABLE_TRACE_CONSOLE_LOG", false),
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
