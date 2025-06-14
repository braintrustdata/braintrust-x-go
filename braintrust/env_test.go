package braintrust

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var envVars = []string{
	"BRAINTRUST_API_KEY",
	"BRAINTRUST_API_URL",
	"BRAINTRUST_APP_URL",
	"BRAINTRUST_TRACE_DEBUG_LOG",
}

func setUpEnvVarTest(t *testing.T) {
	original := make(map[string]string)

	for _, v := range envVars {
		original[v] = os.Getenv(v)
	}

	t.Cleanup(func() {
		for k, v := range original {
			_ = os.Setenv(k, v)
		}
	})
}

func unsetEnvVars() {
	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}
}

func TestGetConfig_DefaultValues(t *testing.T) {
	setUpEnvVarTest(t)

	// Clear environment variables to test defaults
	unsetEnvVars()

	config := GetConfig()
	assert.Equal(t, "", config.APIKey)
	assert.Equal(t, "https://api.braintrust.dev", config.APIURL)
	assert.Equal(t, "https://www.braintrust.dev", config.AppURL)
	assert.Equal(t, false, config.TraceDebugLog)
}

func TestGetConfig_EnvironmentValues(t *testing.T) {
	setUpEnvVarTest(t)

	// Set environment variables
	_ = os.Setenv("BRAINTRUST_API_KEY", "sk-test-key")
	_ = os.Setenv("BRAINTRUST_API_URL", "http://localhost:8000")
	_ = os.Setenv("BRAINTRUST_APP_URL", "http://localhost:3000")
	_ = os.Setenv("BRAINTRUST_TRACE_DEBUG_LOG", "true")

	config := GetConfig()

	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, "http://localhost:8000", config.APIURL)
	assert.Equal(t, "http://localhost:3000", config.AppURL)
	assert.Equal(t, true, config.TraceDebugLog)
}

func TestGetEnvBool(t *testing.T) {
	setUpEnvVarTest(t)

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
