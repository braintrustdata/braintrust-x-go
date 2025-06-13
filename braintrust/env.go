package braintrust

import (
	"os"
	"strings"
)

// Config holds the configuration for Braintrust
type Config struct {
	APIKey        string
	APIURL        string
	AppURL        string
	TraceDebugLog bool
}

// GetConfig returns the Braintrust configuration from environment variables
func GetConfig() Config {
	return Config{
		APIKey:        getEnvString("BRAINTRUST_API_KEY", ""),
		APIURL:        getEnvString("BRAINTRUST_API_URL", "https://api.braintrust.dev"),
		AppURL:        getEnvString("BRAINTRUST_APP_URL", "https://www.braintrust.dev"),
		TraceDebugLog: getEnvBool("BRAINTRUST_TRACE_DEBUG_LOG", false),
	}
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
