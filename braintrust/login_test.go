package braintrust

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogin_WithTestAPIKey tests login with the special test API key
func TestLogin_WithTestAPIKey(t *testing.T) {
	result, err := Login(
		WithAPIKey("___TEST_API_KEY___"),
	)

	require.NoError(t, err)
	assert.Equal(t, "___TEST_API_KEY___", result.LoginToken)
	assert.Equal(t, "test-org-id", result.OrgID)
	assert.Equal(t, "test-org-name", result.OrgName)
	assert.True(t, result.LoggedIn)
}

// TestLogin_WithOptions tests login with multiple options
func TestLogin_WithOptions(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/apikey/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer my-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "my-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	result, err := Login(
		WithAppURL(server.URL),
		WithAPIKey("my-api-key"),
	)

	require.NoError(t, err)
	assert.Equal(t, "my-api-key", result.LoginToken)
	assert.Equal(t, "org-123", result.OrgID)
	assert.Equal(t, "my-org", result.OrgName)
	assert.Equal(t, "https://api.example.com", result.APIURL)
	assert.True(t, result.LoggedIn)
}

// TestLogin_WithOrgName tests login with org name selection
func TestLogin_WithOrgName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-1",
					"name": "org-one",
					"api_url": "https://api1.example.com",
					"proxy_url": "https://proxy1.example.com"
				},
				{
					"id": "org-2",
					"name": "org-two",
					"api_url": "https://api2.example.com",
					"proxy_url": "https://proxy2.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	result, err := Login(
		WithAppURL(server.URL),
		WithAPIKey("test-key"),
		WithOrgName("org-two"),
	)

	require.NoError(t, err)
	assert.Equal(t, "org-2", result.OrgID)
	assert.Equal(t, "org-two", result.OrgName)
	assert.Equal(t, "https://api2.example.com", result.APIURL)
}

// TestLogin_WithEnvironmentVars tests login using environment variables
func TestLogin_WithEnvironmentVars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer env-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-from-env",
					"name": "env-org",
					"api_url": "https://api.env.com",
					"proxy_url": "https://proxy.env.com"
				}
			]
		}`))
	}))
	defer server.Close()

	t.Setenv("BRAINTRUST_API_KEY", "env-api-key")
	t.Setenv("BRAINTRUST_APP_URL", server.URL)

	result, err := Login()

	require.NoError(t, err)
	assert.Equal(t, "env-api-key", result.LoginToken)
	assert.Equal(t, "org-from-env", result.OrgID)
	assert.True(t, result.LoggedIn)
}

// TestLogin_OptionsOverrideEnvironment tests that options override env vars
func TestLogin_OptionsOverrideEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should use the option value, not env var
		assert.Equal(t, "Bearer option-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-option",
					"name": "option-org",
					"api_url": "https://api.option.com",
					"proxy_url": "https://proxy.option.com"
				}
			]
		}`))
	}))
	defer server.Close()

	t.Setenv("BRAINTRUST_API_KEY", "env-key")
	t.Setenv("BRAINTRUST_APP_URL", "http://env-url.com")

	result, err := Login(
		WithAppURL(server.URL),
		WithAPIKey("option-key"),
	)

	require.NoError(t, err)
	assert.Equal(t, "option-key", result.LoginToken)
	assert.Equal(t, "org-option", result.OrgID)
}

// TestLoginResult_String tests the String method for safe console output
func TestLoginResult_String(t *testing.T) {
	result := &LoginResult{
		LoginToken:   "sk-1234567890abcdef",
		OrgID:        "org-123",
		OrgName:      "test-org",
		APIURL:       "https://api.braintrust.dev",
		ProxyURL:     "https://proxy.braintrust.dev",
		AppURL:       "https://www.braintrust.dev",
		AppPublicURL: "https://public.braintrust.dev",
		LoggedIn:     true,
	}

	str := result.String()

	// Token should be redacted
	assert.Contains(t, str, "<redacted>")
	assert.NotContains(t, str, "sk-1234567890abcdef")

	// Should contain org info
	assert.Contains(t, str, "test-org")
	assert.Contains(t, str, "org-123")

	// Should contain all URLs
	assert.Contains(t, str, "https://api.braintrust.dev")
	assert.Contains(t, str, "https://proxy.braintrust.dev")
	assert.Contains(t, str, "https://www.braintrust.dev")
	assert.Contains(t, str, "https://public.braintrust.dev")
}

// TestLoginResult_String_ShortToken tests String with a short token
func TestLoginResult_String_ShortToken(t *testing.T) {
	result := &LoginResult{
		LoginToken: "short",
		OrgName:    "test-org",
		OrgID:      "org-123",
		APIURL:     "https://api.test.com",
		AppURL:     "https://app.test.com",
	}

	str := result.String()

	// Short token should be redacted
	assert.Contains(t, str, "<redacted>")
	assert.NotContains(t, str, "short")
}
