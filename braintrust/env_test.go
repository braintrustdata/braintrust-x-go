package braintrust

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_DefaultValues(t *testing.T) {
	// Clear environment variables to test defaults
	originalAPIKey := os.Getenv("BRAINTRUST_API_KEY")
	originalAPIURL := os.Getenv("BRAINTRUST_API_URL")
	originalAppURL := os.Getenv("BRAINTRUST_APP_URL")
	originalTraceDebugLog := os.Getenv("BRAINTRUST_TRACE_DEBUG_LOG")

	_ = os.Unsetenv("BRAINTRUST_API_KEY")
	_ = os.Unsetenv("BRAINTRUST_API_URL")
	_ = os.Unsetenv("BRAINTRUST_APP_URL")
	_ = os.Unsetenv("BRAINTRUST_TRACE_DEBUG_LOG")

	defer func() {
		// Restore original values
		if originalAPIKey != "" {
			_ = os.Setenv("BRAINTRUST_API_KEY", originalAPIKey)
		}
		if originalAPIURL != "" {
			_ = os.Setenv("BRAINTRUST_API_URL", originalAPIURL)
		}
		if originalAppURL != "" {
			_ = os.Setenv("BRAINTRUST_APP_URL", originalAppURL)
		}
		if originalTraceDebugLog != "" {
			_ = os.Setenv("BRAINTRUST_TRACE_DEBUG_LOG", originalTraceDebugLog)
		}
	}()

	config := GetConfig()

	assert.Equal(t, "", config.APIKey)
	assert.Equal(t, "https://api.braintrust.dev", config.APIURL)
	assert.Equal(t, "https://www.braintrust.dev", config.AppURL)
	assert.Equal(t, false, config.TraceDebugLog)
}

func TestGetConfig_EnvironmentValues(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("BRAINTRUST_API_KEY", "sk-test-key")
	_ = os.Setenv("BRAINTRUST_API_URL", "http://localhost:8000")
	_ = os.Setenv("BRAINTRUST_APP_URL", "http://localhost:3000")
	_ = os.Setenv("BRAINTRUST_TRACE_DEBUG_LOG", "true")

	defer func() {
		_ = os.Unsetenv("BRAINTRUST_API_KEY")
		_ = os.Unsetenv("BRAINTRUST_API_URL")
		_ = os.Unsetenv("BRAINTRUST_APP_URL")
		_ = os.Unsetenv("BRAINTRUST_TRACE_DEBUG_LOG")
	}()

	config := GetConfig()

	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, "http://localhost:8000", config.APIURL)
	assert.Equal(t, "http://localhost:3000", config.AppURL)
	assert.Equal(t, true, config.TraceDebugLog)
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{"true lowercase", "true", false, true},
		{"TRUE uppercase", "TRUE", false, true},
		{"True mixed case", "True", false, true},
		{"false lowercase", "false", true, false},
		{"FALSE uppercase", "FALSE", true, false},
		{"random value", "yes", false, false},
		{"empty value", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_VAR"
			if tt.envValue != "" {
				_ = os.Setenv(key, tt.envValue)
			} else {
				_ = os.Unsetenv(key)
			}
			defer func() { _ = os.Unsetenv(key) }()

			result := getEnvBool(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
