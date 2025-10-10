package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
)

const (
	// TestAPIKey is a special API key used for testing
	TestAPIKey = "___TEST_API_KEY___"
	// DefaultAppURL is the default Braintrust app URL
	DefaultAppURL = "https://www.braintrust.dev"
)

// loginError wraps an error with HTTP status code information
type loginError struct {
	err        error
	statusCode int
}

func (e *loginError) Error() string {
	return e.err.Error()
}

func (e *loginError) Unwrap() error {
	return e.err
}

// cacheKey uniquely identifies a login session
type cacheKey struct {
	APIKey  string
	OrgName string
}

type loginCache struct {
	mu    sync.RWMutex
	cache map[cacheKey]*State
}

func newLoginCache() *loginCache {
	return &loginCache{
		cache: make(map[cacheKey]*State),
	}
}

func (c *loginCache) get(key cacheKey) (*State, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.cache[key]
	return result, ok
}

func (c *loginCache) set(key cacheKey, result *State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = result
}

func (c *loginCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[cacheKey]*State)
}

// globalLoginCache is the package-level cache instance
var globalLoginCache = newLoginCache()

// Options contains options for the Login function
type Options struct {
	// AppURL is the URL of the Braintrust app
	AppURL string
	// AppPublicURL is the public URL of the Braintrust app
	AppPublicURL string
	// APIKey is the API key to use
	APIKey string
	// OrgName is the name of a specific organization to connect to (optional)
	OrgName string
}

type State struct {
	LoginToken   string
	OrgID        string
	OrgName      string
	APIURL       string
	ProxyURL     string
	AppURL       string
	AppPublicURL string
	LoggedIn     bool
}

// OrgInfo represents organization information from the login response
type OrgInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	APIURL   string `json:"api_url"`
	ProxyURL string `json:"proxy_url"`
}

// loginResponse represents the response from the login API
type loginResponse struct {
	OrgInfo []OrgInfo `json:"org_info"`
}

// makeLoginRequest makes an HTTP request to the login API and returns the parsed response.
func makeLoginRequest(appURL, apiKey string) (*loginResponse, error) {
	req, err := http.NewRequest("POST", appURL+"/api/apikey/login", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating login request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making login request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		maskedKey := maskAPIKey(apiKey)
		return nil, &loginError{
			err:        fmt.Errorf("invalid API key %s: [%d]", maskedKey, resp.StatusCode),
			statusCode: resp.StatusCode,
		}
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("error decoding login response: %w", err)
	}

	return &loginResp, nil
}

// Login authenticates with the Braintrust API and returns login information.
// It implements the same logic as the Python SDK's login() function.
// Results are cached by API key, app URL, and org name to avoid redundant API calls.
func Login(opts Options) (*State, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if opts.AppURL == "" {
		return nil, fmt.Errorf("app URL is required")
	}

	apiKey := opts.APIKey
	appURL := opts.AppURL
	appPublicURL := opts.AppPublicURL
	if appPublicURL == "" {
		appPublicURL = appURL
	}
	orgName := opts.OrgName

	log.Debugf("Login: attempting login with API key %s, org %q, app URL %s", maskAPIKey(apiKey), orgName, appURL)

	// Check cache first
	key := cacheKey{
		APIKey:  apiKey,
		OrgName: orgName,
	}
	if cached, ok := globalLoginCache.get(key); ok && cached.LoggedIn {
		log.Debugf("Login: using cached result for API key %s, org %q", maskAPIKey(apiKey), orgName)
		return cached, nil
	}

	result := &State{
		AppURL:       appURL,
		AppPublicURL: appPublicURL,
		LoginToken:   apiKey,
	}

	// Handle test API key
	if apiKey == TestAPIKey {
		testOrgInfo := []OrgInfo{
			{
				ID:       "test-org-id",
				Name:     getOrgNameOrDefault(orgName, "test-org-name"),
				APIURL:   "https://api.braintrust.ai",
				ProxyURL: "https://proxy.braintrust.ai",
			},
		}
		if err := selectOrg(result, testOrgInfo, orgName); err != nil {
			return nil, err
		}
		result.LoggedIn = true
		globalLoginCache.set(key, result)
		return result, nil
	}

	// Make API request to login
	loginResp, err := makeLoginRequest(appURL, apiKey)
	if err != nil {
		return nil, err
	}

	// Check and select organization
	if err := selectOrg(result, loginResp.OrgInfo, orgName); err != nil {
		return nil, err
	}

	result.LoggedIn = true
	globalLoginCache.set(key, result)
	log.Debugf("Login: successfully logged in to org %q (%s)", result.OrgName, result.OrgID)
	return result, nil
}

// selectOrg selects the appropriate organization from the org_info list
func selectOrg(result *State, orgInfo []OrgInfo, orgName string) error {
	if len(orgInfo) == 0 {
		return fmt.Errorf("this user is not part of any organizations")
	}

	// Find the matching org
	for _, org := range orgInfo {
		if orgName == "" || org.Name == orgName {
			result.OrgID = org.ID
			result.OrgName = org.Name
			result.APIURL = org.APIURL
			result.ProxyURL = org.ProxyURL
			return nil
		}
	}

	// If we get here, the org was not found
	orgNames := make([]string, len(orgInfo))
	for i, org := range orgInfo {
		orgNames[i] = org.Name
	}
	return fmt.Errorf("organization %q not found. Must be one of: %s", orgName, strings.Join(orgNames, ", "))
}

// getOrgNameOrDefault returns orgName if not empty, otherwise returns defaultName
func getOrgNameOrDefault(orgName, defaultName string) string {
	if orgName != "" {
		return orgName
	}
	return defaultName
}

// maskAPIKey masks an API key for safe display
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 6 {
		return "<redacted>"
	}
	return apiKey[:3] + "..." + apiKey[len(apiKey)-3:]
}

// Logout clears all cached login sessions
func Logout() {
	log.Debugf("Logout: clearing all cached login sessions")
	globalLoginCache.clear()
}

// GetState retrieves a cached login state by API key and optional org name.
// Returns the state and a boolean indicating if it was found.
func GetState(apiKey string, orgName string) (State, bool) {
	key := cacheKey{
		APIKey:  apiKey,
		OrgName: orgName,
	}

	result, ok := globalLoginCache.get(key)
	if !ok || !result.LoggedIn {
		return State{}, false
	}
	return *result, true
}

// isRetryableError determines if an error should trigger a retry.
// Returns true for 5xx errors and network errors, false for 4xx errors and validation errors.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a loginError with status code
	var le *loginError
	if errors.As(err, &le) {
		// Retry on 5xx errors (server errors)
		if le.statusCode >= 500 && le.statusCode < 600 {
			return true
		}
		// Don't retry on 4xx errors (client errors)
		if le.statusCode >= 400 && le.statusCode < 500 {
			return false
		}
	}

	// Check for validation errors - don't retry
	if strings.Contains(err.Error(), "API key is required") ||
		strings.Contains(err.Error(), "app URL is required") ||
		strings.Contains(err.Error(), "organization") ||
		strings.Contains(err.Error(), "not part of any organizations") {
		return false
	}

	// Network errors, timeouts, and other errors - retry
	return true
}

// LoginUntilSuccess attempts to login with exponential backoff retry.
// It retries indefinitely on 5xx errors and network errors, but returns immediately on 4xx errors.
// Backoff starts at 10ms and doubles each attempt, capped at 10 seconds.
func LoginUntilSuccess(opts Options) (*State, error) {
	attempt := 0
	for {
		result, err := Login(opts)
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			log.Debugf("LoginUntilSuccess: non-retryable error: %v", err)
			return nil, err
		}

		// Calculate exponential backoff: 10ms * 2^attempt, capped at 10 seconds
		delay := time.Duration(10*math.Pow(2, float64(attempt))) * time.Millisecond
		maxDelay := 10 * time.Second
		if delay > maxDelay {
			delay = maxDelay
		}
		log.Debugf("LoginUntilSuccess: attempt %d failed: %v, retrying in %v", attempt+1, err, delay)
		time.Sleep(delay)
		attempt++
	}
}
