package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
)

// Experiment represents an experiment from the API
type Experiment struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	ProjectID string                 `json:"project_id"`
	Tags      []string               `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// RegisterExperimentOpts contains optional parameters for registering an experiment
type RegisterExperimentOpts struct {
	Tags     []string
	Metadata map[string]interface{}
	Update   bool // If true, allow reusing existing experiment
}

// RegisterExperiment creates or gets an experiment.
func RegisterExperiment(ctx context.Context, cfg *config.Config, session *auth.Session, name string, projectID string, opts RegisterExperimentOpts) (*Experiment, error) {
	if name == "" {
		return nil, fmt.Errorf("experiment name is required")
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	// Create experiment request
	reqBody := map[string]interface{}{
		"project_id": projectID,
		"name":       name,
		"ensure_new": !opts.Update,
	}
	if len(opts.Tags) > 0 {
		reqBody["tags"] = opts.Tags
	}
	if opts.Metadata != nil {
		reqBody["metadata"] = opts.Metadata
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Get API URL from config or session
	apiURL := cfg.APIURL
	if apiURL == "" {
		// Try to get from session if login is complete
		if ok, info := session.Info(); ok && info.APIURL != "" {
			apiURL = info.APIURL
		} else {
			apiURL = "https://api.braintrust.ai"
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1/experiment", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// Make request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result Experiment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}
