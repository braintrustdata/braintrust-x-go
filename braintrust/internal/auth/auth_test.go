package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogin_WithTestAPIKey tests login with the special test API key
func TestLogin_WithTestAPIKey(t *testing.T) {
	result, err := Login(Options{
		AppURL: "https://www.braintrust.dev",
		APIKey: "___TEST_API_KEY___",
	})

	require.NoError(t, err)
	assert.Equal(t, "___TEST_API_KEY___", result.LoginToken)
	assert.Equal(t, "test-org-id", result.OrgID)
	assert.Equal(t, "test-org-name", result.OrgName)
	assert.Equal(t, "https://api.braintrust.ai", result.APIURL)
	assert.Equal(t, "https://proxy.braintrust.ai", result.ProxyURL)
	assert.True(t, result.LoggedIn)
}

// TestLogin_WithValidAPIKey tests login with a valid API key
func TestLogin_WithValidAPIKey(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		assert.Equal(t, "/api/apikey/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	result, err := Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})

	require.NoError(t, err)
	assert.Equal(t, "test-api-key", result.LoginToken)
	assert.Equal(t, "org-123", result.OrgID)
	assert.Equal(t, "test-org", result.OrgName)
	assert.Equal(t, "https://api.example.com", result.APIURL)
	assert.Equal(t, "https://proxy.example.com", result.ProxyURL)
	assert.True(t, result.LoggedIn)
}

// TestLogin_WithInvalidAPIKey tests login with an invalid API key
func TestLogin_WithInvalidAPIKey(t *testing.T) {
	// Create a mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid API key"))
	}))
	defer server.Close()

	_, err := Login(Options{
		AppURL: server.URL,
		APIKey: "invalid-key",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
}

// TestLogin_OrgSelection tests selecting a specific org by name
func TestLogin_OrgSelection(t *testing.T) {
	// Create a mock server with multiple orgs
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

	result, err := Login(Options{
		AppURL:  server.URL,
		APIKey:  "test-api-key",
		OrgName: "org-two",
	})

	require.NoError(t, err)
	assert.Equal(t, "org-2", result.OrgID)
	assert.Equal(t, "org-two", result.OrgName)
	assert.Equal(t, "https://api2.example.com", result.APIURL)
}

// TestLogin_OrgNotFound tests error when specified org doesn't exist
func TestLogin_OrgNotFound(t *testing.T) {
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
				}
			]
		}`))
	}))
	defer server.Close()

	_, err := Login(Options{
		AppURL:  server.URL,
		APIKey:  "test-api-key",
		OrgName: "non-existent-org",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization")
	assert.Contains(t, err.Error(), "non-existent-org")
}

// TestLogin_NoAPIKey tests error when no API key is provided
func TestLogin_NoAPIKey(t *testing.T) {
	_, err := Login(Options{
		AppURL: "https://www.braintrust.dev",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

// TestLogin_NoAppURL tests error when no app URL is provided
func TestLogin_NoAppURL(t *testing.T) {
	_, err := Login(Options{
		APIKey: "test-key",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app URL")
}

// TestLogin_CacheHit tests that subsequent logins with same params use cache
func TestLogin_CacheHit(t *testing.T) {
	// Clear cache before test
	Logout()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	// First login - should call server
	result1, err := Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "First login should call server")

	// Second login with same params - should use cache
	result2, err := Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "Second login should not call server")

	// Results should be identical
	assert.Equal(t, result1.LoginToken, result2.LoginToken)
	assert.Equal(t, result1.OrgID, result2.OrgID)
	assert.Equal(t, result1.OrgName, result2.OrgName)
}

// TestLogin_CacheMiss_DifferentOrgName tests that different org names don't hit cache
func TestLogin_CacheMiss_DifferentOrgName(t *testing.T) {
	// Clear cache before test
	Logout()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
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

	// Login with org-one
	result1, err := Login(Options{
		AppURL:  server.URL,
		APIKey:  "test-api-key",
		OrgName: "org-one",
	})
	require.NoError(t, err)
	assert.Equal(t, "org-1", result1.OrgID)
	assert.Equal(t, 1, callCount)

	// Login with org-two - should not use cache
	result2, err := Login(Options{
		AppURL:  server.URL,
		APIKey:  "test-api-key",
		OrgName: "org-two",
	})
	require.NoError(t, err)
	assert.Equal(t, "org-2", result2.OrgID)
	assert.Equal(t, 2, callCount, "Different org name should not hit cache")
}

// TestLogout_ClearsCache tests that Logout clears all cached results
func TestLogout_ClearsCache(t *testing.T) {
	// Clear cache before test
	Logout()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	// First login
	_, err := Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second login - should use cache
	_, err = Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Logout to clear cache
	Logout()

	// Third login - should call server again
	_, err = Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Login after Logout should call server")
}

// TestGetState tests retrieving cached login state
func TestGetState(t *testing.T) {
	// Clear cache before test
	Logout()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	// GetState should return false before login
	_, ok := GetState("test-api-key", "")
	assert.False(t, ok)

	// Login to populate cache
	loginResult, err := Login(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})
	require.NoError(t, err)

	// GetState should now return cached result
	state, ok := GetState("test-api-key", "")
	require.True(t, ok)
	assert.Equal(t, loginResult.LoginToken, state.LoginToken)
	assert.Equal(t, loginResult.OrgID, state.OrgID)
	assert.Equal(t, loginResult.OrgName, state.OrgName)

	// GetState with wrong org name should return false
	_, ok = GetState("test-api-key", "wrong-org")
	assert.False(t, ok)

	// Logout should clear cache
	Logout()
	_, ok = GetState("test-api-key", "")
	assert.False(t, ok)
}

// TestLoginUntilSuccess_ImmediateSuccess tests that LoginUntilSuccess succeeds on first try
func TestLoginUntilSuccess_ImmediateSuccess(t *testing.T) {
	// Clear cache before test
	Logout()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	result, err := LoginUntilSuccess(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})

	require.NoError(t, err)
	assert.Equal(t, "test-api-key", result.LoginToken)
	assert.Equal(t, "org-123", result.OrgID)
}

// TestLoginUntilSuccess_RetryOn500 tests that LoginUntilSuccess retries on 500 errors
func TestLoginUntilSuccess_RetryOn500(t *testing.T) {
	// Clear cache before test
	Logout()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// Return 500 on first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal Server Error"))
			return
		}
		// Succeed on third attempt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"org_info": [
				{
					"id": "org-123",
					"name": "test-org",
					"api_url": "https://api.example.com",
					"proxy_url": "https://proxy.example.com"
				}
			]
		}`))
	}))
	defer server.Close()

	result, err := LoginUntilSuccess(Options{
		AppURL: server.URL,
		APIKey: "test-api-key",
	})

	require.NoError(t, err)
	assert.Equal(t, 3, callCount, "Should have made 3 attempts")
	assert.Equal(t, "org-123", result.OrgID)
}

// TestLoginUntilSuccess_NoRetryOn401 tests that LoginUntilSuccess doesn't retry on 401
func TestLoginUntilSuccess_NoRetryOn401(t *testing.T) {
	// Clear cache before test
	Logout()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	_, err := LoginUntilSuccess(Options{
		AppURL: server.URL,
		APIKey: "invalid-key",
	})

	require.Error(t, err)
	assert.Equal(t, 1, callCount, "Should have made only 1 attempt (no retry on 401)")
	assert.Contains(t, err.Error(), "invalid API key")
}

// TestLoginUntilSuccess_NoRetryOnValidationError tests that LoginUntilSuccess doesn't retry on validation errors
func TestLoginUntilSuccess_NoRetryOnValidationError(t *testing.T) {
	_, err := LoginUntilSuccess(Options{
		AppURL: "https://example.com",
		APIKey: "", // Missing API key
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key is required")
}
