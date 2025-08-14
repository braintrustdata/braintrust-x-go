// Package functions provides functionality for invoking remote Braintrust
// functions and scorers.
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

// GetScorer returns the most recent scorer with the given name and slug. It will
// return an error if no scorer is found.
func GetScorer[I, R any](projectName, slug string) (eval.Scorer[I, R], error) {
	opts := Opts{
		ProjectName: projectName,
		Slug:        slug,
		Limit:       1,
		// No version specified = most recent
	}
	return QueryScorer[I, R](opts)
}

// QueryScorer returns the most recent Scorer matching the given Opts.
func QueryScorer[I, R any](opts Opts) (eval.Scorer[I, R], error) {
	// Ensure limit is 1 for single scorer
	opts.Limit = 1

	scorers, err := QueryScorers[I, R](opts)
	if err != nil {
		return nil, err
	}

	if len(scorers) == 0 {
		return nil, fmt.Errorf("no functions found matching the criteria")
	}

	return scorers[0], nil
}

// QueryScorers provides flexible querying for multiple scorers (user controls limit).
func QueryScorers[I, R any](opts Opts) ([]eval.Scorer[I, R], error) {
	// Query all matching functions
	functions, err := queryFunctions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query functions: %w", err)
	}

	// Return empty list if no functions found (not an error for QueryScorers)
	if len(functions) == 0 {
		return []eval.Scorer[I, R]{}, nil
	}

	// Create scorers for each function
	scorers := make([]eval.Scorer[I, R], len(functions))
	for i, function := range functions {
		scorers[i] = newFunctionScorer[I, R](function.Name, function.ID)
	}

	return scorers, nil
}

// Opts provides flexible options for querying Braintrust functions
type Opts struct {
	// Project identity (either/or)
	ProjectName string // Filter by project name
	ProjectID   string // Filter by specific project ID

	// Function identity (either/or)
	Slug         string // Filter by function slug
	FunctionName string // Filter by function name

	// Direct bypass (overrides all above)
	FunctionID string // Use specific function ID directly

	// Query modifiers
	Version string // Specific function version
	Limit   int    // Max results (default: no limit for QueryScorers)
}

// Function represents a Braintrust function.
type Function struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	FunctionType string `json:"function_type"`
}

// queryFunctions queries the Braintrust API for functions matching the options
func queryFunctions(opts Opts) ([]Function, error) {
	// If function ID is provided directly, create a mock function entry
	if opts.FunctionID != "" {
		return []Function{{
			ID:   opts.FunctionID,
			Name: opts.FunctionID, // Use ID as name
			Slug: opts.FunctionID, // Use ID as slug fallback
		}}, nil
	}

	// Otherwise query the API
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return nil, fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Build the URL with query parameters
	baseURL := fmt.Sprintf("%s/v1/function", config.APIURL)
	params := url.Values{}

	if opts.ProjectName != "" {
		params.Add("project_name", opts.ProjectName)
	}
	if opts.ProjectID != "" {
		params.Add("project_id", opts.ProjectID)
	}
	if opts.Slug != "" {
		params.Add("slug", opts.Slug)
	}
	if opts.FunctionName != "" {
		params.Add("function_name", opts.FunctionName)
	}
	if opts.Version != "" {
		params.Add("version", opts.Version)
	}

	// Add limit if specified (for QueryScorers, this could be 0 = no limit)
	if opts.Limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", opts.Limit))
	}

	fullURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response struct {
		Objects []Function `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Objects, nil
}

// createFunction creates a new function via the Braintrust API
func createFunction(projectName, name, slug, description string, functionData map[string]any) (string, error) {
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// First resolve project name to project ID
	projectID, err := resolveProjectID(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project ID: %w", err)
	}

	// Build the request payload
	payload := map[string]any{
		"project_id":    projectID,
		"name":          name,
		"slug":          slug,
		"function_type": "scorer",
		"function_data": functionData,
	}

	if description != "" {
		payload["description"] = description
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the API request
	url := fmt.Sprintf("%s/v1/function", config.APIURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var createdFunction Function
	if err := json.NewDecoder(resp.Body).Decode(&createdFunction); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return createdFunction.ID, nil
}

// createFunctionWithPromptData creates a function with full prompt_data structure
func createFunctionWithPromptData(projectName, name, slug, description string, promptData map[string]any) (string, error) {
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// First resolve project name to project ID
	projectID, err := resolveProjectID(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project ID: %w", err)
	}

	// Build the request payload with prompt_data
	payload := map[string]any{
		"project_id":    projectID,
		"name":          name,
		"slug":          slug,
		"function_type": "scorer",
		"function_data": map[string]any{
			"type": "prompt",
		},
		"prompt_data": promptData,
	}

	if description != "" {
		payload["description"] = description
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the API request
	url := fmt.Sprintf("%s/v1/function", config.APIURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var createdFunction Function
	if err := json.NewDecoder(resp.Body).Decode(&createdFunction); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return createdFunction.ID, nil
}

// resolveProjectID resolves a project name to a project ID via the Braintrust API
func resolveProjectID(projectName string) (string, error) {
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Query projects API to find project by name
	url := fmt.Sprintf("%s/v1/project?project_name=%s", config.APIURL, projectName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response struct {
		Objects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Objects) == 0 {
		return "", fmt.Errorf("project not found: %s", projectName)
	}

	return response.Objects[0].ID, nil
}

type functionScorer[I, R any] struct {
	name       string
	functionID string
	client     *http.Client
}

// newFunctionScorer creates a new function scorer with a default HTTP client
func newFunctionScorer[I, R any](name, functionID string) *functionScorer[I, R] {
	return &functionScorer[I, R]{
		name:       name,
		functionID: functionID,
		client:     &http.Client{},
	}
}

func (f *functionScorer[I, R]) Name() string {
	return f.name
}

func (f *functionScorer[I, R]) Run(ctx context.Context, input I, expected, result R, meta eval.Metadata) (eval.Scores, error) {
	scoringArgs := map[string]any{
		"input":    input,
		"expected": expected,
		"output":   result,
	}

	payload := map[string]any{
		"input":    scoringArgs,
		"metadata": meta,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Get Braintrust configuration
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return nil, fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Use the correct API endpoint for function invocation
	url := fmt.Sprintf("%s/v1/function/%s/invoke", config.APIURL, f.functionID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke function: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("function invocation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response struct {
		Score float64 `json:"score"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return the score
	return eval.Scores{{
		Name:  f.name,
		Score: response.Score,
	}}, nil
}
