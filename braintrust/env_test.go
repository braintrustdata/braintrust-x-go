package braintrust

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_WithDefaultProjectID(t *testing.T) {
	config := GetConfig(WithDefaultProjectID("my-project"))
	assert.Equal(t, "my-project", config.DefaultProjectID)
}

func TestConfigString(t *testing.T) {
	var apiKey string
	for range 10 {
		apiKey += "a"
		config := GetConfig(WithAPIKey(apiKey))
		assert.Contains(t, config.String(), "Braintrust Config")
	}
}

func TestGetConfig_DefaultValues(t *testing.T) {
	envVars := []string{
		"BRAINTRUST_API_KEY",
		"BRAINTRUST_API_URL",
		"BRAINTRUST_APP_URL",
		"BRAINTRUST_ENABLE_TRACE_CONSOLE_LOG",
		"BRAINTRUST_DEFAULT_PROJECT_ID",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
	}
	config := GetConfig()
	assert.Equal(t, "", config.APIKey)
	assert.Equal(t, "https://api.braintrust.dev", config.APIURL)
	assert.Equal(t, "https://www.braintrust.dev", config.AppURL)
	assert.Equal(t, false, config.EnableTraceConsoleLog)
	assert.Equal(t, "", config.DefaultProjectID)
}

func TestGetConfig_EnvironmentValues(t *testing.T) {
	// Set environment variables
	t.Setenv("BRAINTRUST_API_KEY", "sk-test-key")
	t.Setenv("BRAINTRUST_API_URL", "http://localhost:8000")
	t.Setenv("BRAINTRUST_APP_URL", "http://localhost:3000")
	t.Setenv("BRAINTRUST_ENABLE_TRACE_CONSOLE_LOG", "true")
	t.Setenv("BRAINTRUST_DEFAULT_PROJECT_ID", "my-project")

	config := GetConfig()

	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, "http://localhost:8000", config.APIURL)
	assert.Equal(t, "http://localhost:3000", config.AppURL)
	assert.Equal(t, true, config.EnableTraceConsoleLog)
	assert.Equal(t, "my-project", config.DefaultProjectID)
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
				t.Setenv(key, tt.envValue)
			} else {
				t.Setenv(key, "")
			}

			result := getEnvBool(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
