// Package api provides client functionality for interacting with the Braintrust API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/braintrust/braintrust-x-go/braintrust"
)

// MANU_COMMENT: In your experience, what is the best way to maintain rich
// typespec definitions across multiple language APIs. I have a pipe dream where
// we maintain the source of truth for all of our APIs as zod typespecs, and
// then in theory we could autogenerate type definitions in other languages from
// these. We already sort-of do this in python.
//
// The pros are that we get nice things like per-field documentation
// synchronized across languages. But on the other hand, it's unlikely to be
// trivial to port generic zod typespecs over to every language, and we end up
// implementing a compiler.

// ExperimentRequest represents the request payload for creating an experiment
// MANU_COMMENT: Maybe RegisterExperimentRequest?
type ExperimentRequest struct {
	ProjectID string `json:"project_id"`
	// MANU_COMMENT: Should we omitempty these too? I imagine all optional args
	// in the REST API should be marked omitempty here. Otherwise we'll create
	// an empty-valued experiment.
	//
	// Also, should we use `omitzero` instead of `omitempty`?
	// (https://tip.golang.org/doc/go1.24#encodingjsonpkgencodingjson)
	Name      string `json:"name"`
	EnsureNew bool   `json:"ensure_new"`
}

// Experiment represents an experiment from the API
type Experiment struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
}

// RegisterExperiment creates a new experiment via the Braintrust API

// MANU_COMMENT: Should we use struct args for these functions? I worry that if
// the set of args grows large like it is in python then a giant positional
// arglist will become unwieldy. I guess you're doing that in the dataset API.

// MANU_COMMENT: It feels a little bit confusing to have separate
// RegisterExperiment and GetOrCreateExperimentMethods. I imagine users will
// just need to use GetOrCreateExperiment?

// MANU_COMMENT: In the python and TS sdks, we do this thing where the objects
// returned by `init/init_dataset/etc` are lazily-initialized. Meaning we return
// something immediately and resolve the experiment against the control plane
// only in the background logger. That makes all of these inits non-blocking.
// Should we do the same in go?
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
