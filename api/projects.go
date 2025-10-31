// Package api provides functions for interacting with the Braintrust API.
// This package works with the client-based architecture and requires passing
// config and session explicitly.
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

// Project represents a project from the API
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RegisterProject creates or gets a project by name.
// If a project with the given name already exists, it returns that project.
func RegisterProject(ctx context.Context, cfg *config.Config, session *auth.Session, name string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}

	// Create project request
	reqBody := map[string]interface{}{
		"name": name,
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
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1/project", bytes.NewBuffer(jsonData))
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

	var result Project
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}
