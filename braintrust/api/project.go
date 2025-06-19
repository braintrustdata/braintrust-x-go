package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/braintrust/braintrust-x-go/braintrust"
)

// ProjectRequest represents the request payload for creating a project
type ProjectRequest struct {
	Name    string `json:"name"`
	OrgName string `json:"org_name,omitempty"`
}

// Project represents a project from the API
type Project struct {
	ID        string            `json:"id"`
	OrgID     string            `json:"org_id"`
	Name      string            `json:"name"`
	Created   time.Time         `json:"created"`
	DeletedAt *time.Time        `json:"deleted_at,omitempty"`
	UserID    string            `json:"user_id"`
	Settings  map[string]string `json:"settings,omitempty"`
}

// RegisterProject creates a new project via the Braintrust API.
// If a project with the same name already exists, returns the existing project.
func RegisterProject(name string) (*Project, error) {
	req := ProjectRequest{
		Name: name,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	httpReq, err := http.NewRequest("POST", config.APIURL+"/v1/project", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result Project
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}
