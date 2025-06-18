// Package api provides client functionality for interacting with the Braintrust API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/braintrust/braintrust-x-go/braintrust"
)

// ExperimentRequest represents the request payload for creating an experiment
type ExperimentRequest struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	EnsureNew bool   `json:"ensure_new"`
}

// Experiment represents an experiment from the API
type Experiment struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
}

// RegisterExperiment creates a new experiment via the Braintrust API
func RegisterExperiment(name string, projectID string) (*Experiment, error) {
	req := ExperimentRequest{
		ProjectID: projectID,
		Name:      name,
		EnsureNew: true,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	httpReq, err := http.NewRequest("POST", config.APIURL+"/v1/experiment", bytes.NewBuffer(jsonData))
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result Experiment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}

// GetOrCreateExperiment takes an experiment name and project name,
// registers/gets the project, then registers/gets the experiment,
// and returns the experiment ID
func GetOrCreateExperiment(experimentName, projectName string) (string, error) {
	// First, register/get the project
	project, err := RegisterProject(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to register project %q: %w", projectName, err)
	}

	// Then register/get the experiment
	experiment, err := RegisterExperiment(experimentName, project.ID)
	if err != nil {
		return "", fmt.Errorf("failed to register experiment %q: %w", experimentName, err)
	}

	return experiment.ID, nil
}
