package braintrust

import (
	"fmt"

	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/auth"
)

// LoginResult contains the result of a login operation.
type LoginResult struct {
	LoginToken   string
	OrgID        string
	OrgName      string
	APIURL       string
	ProxyURL     string
	AppURL       string
	AppPublicURL string
	LoggedIn     bool
}

// String returns a safe string representation of the login result with the token masked.
func (r LoginResult) String() string {
	token := "<not set>"
	if r.LoginToken != "" {
		token = "<redacted>"
	}

	return fmt.Sprintf(`Login Result:
  Organization: %s (ID: %s)
  API URL: %s
  Proxy URL: %s
  App URL: %s
  App Public URL: %s
  Token: %s`,
		r.OrgName,
		r.OrgID,
		r.APIURL,
		r.ProxyURL,
		r.AppURL,
		r.AppPublicURL,
		token,
	)
}

// Login authenticates with the Braintrust API and returns login information.
// It accepts Options to configure the login behavior (WithAPIKey, WithAppURL, WithOrgName).
// If no options are provided, it will use environment variables
// (BRAINTRUST_API_KEY, BRAINTRUST_APP_URL, BRAINTRUST_ORG_NAME).
//
// Example:
//
//	result, err := braintrust.Login(
//	    braintrust.WithAPIKey("your-api-key"),
//	    braintrust.WithOrgName("your-org"),
//	)
func Login(opts ...Option) (*LoginResult, error) {
	cfg := GetConfig(opts...)

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("could not login to Braintrust: API key is required. Set BRAINTRUST_API_KEY environment variable or pass WithAPIKey option")
	}

	// Call internal login with the config values
	internalOpts := auth.Options{
		AppURL:       cfg.AppURL,
		AppPublicURL: cfg.AppURL, // Use AppURL for both
		APIKey:       cfg.APIKey,
		OrgName:      cfg.OrgName,
	}

	result, err := auth.Login(internalOpts)
	if err != nil {
		return nil, err
	}

	// Convert internal.LoginResult to braintrust.LoginResult
	return &LoginResult{
		LoginToken:   result.LoginToken,
		OrgID:        result.OrgID,
		OrgName:      result.OrgName,
		APIURL:       result.APIURL,
		ProxyURL:     result.ProxyURL,
		AppURL:       result.AppURL,
		AppPublicURL: result.AppPublicURL,
		LoggedIn:     result.LoggedIn,
	}, nil
}
