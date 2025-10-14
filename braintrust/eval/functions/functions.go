// Package functions provides functionality for invoking remote Braintrust
// functions and scorers.
//
// Braintrust functions include prompts, tools, and scorers that are hosted
// in Braintrust and can be invoked remotely.
//
// # Using Prompts in Evals
//
// Use GetTask to create a task function from a hosted prompt.
// The server handles all prompt templating with the input variables from each
// eval case.
//
//	task := functions.GetTask[string, string](functions.Opts{
//	    Project: "my-project",
//	    Slug:    "my-prompt",
//	})
//
// # Using Scorers
//
// Use GetScorer or QueryScorer to get a hosted scorer:
//
//	scorer, err := functions.GetScorer[string, string]("my-project", "my-scorer")
//
// See the package example for a complete usage demonstration.
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

// GetScorer returns the most recent scorer with the given name and slug. It will
// return an error if no scorer is found.
func GetScorer[I, R any](projectName, slug string) (eval.Scorer[I, R], error) {
	opts := Opts{
		Project: projectName,
		Slug:    slug,
		Limit:   1,
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
	// FIXME: Accept context.Context as first parameter to allow cancellation
	// This would be a breaking change, so using context.Background() for now
	ctx := context.Background()

	// Query all matching functions
	functions, err := queryFunctions(ctx, opts)
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
	Project   string // Filter by project name
	ProjectID string // Filter by specific project ID

	// Function identity (either/or)
	Slug         string // Filter by function slug
	FunctionName string // Filter by function name

	// Direct bypass (overrides all above)
	FunctionID string // Use specific function ID directly

	// Query modifiers
	Version     string // Specific function version
	Environment string // Environment to load (dev/staging/production)
	Limit       int    // Max results (default: no limit for QueryScorers)
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
func queryFunctions(ctx context.Context, opts Opts) ([]Function, error) {
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

	if opts.Project != "" {
		params.Add("project_name", opts.Project)
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
	if opts.Environment != "" {
		params.Add("environment", opts.Environment)
	}

	// Add limit if specified (for QueryScorers, this could be 0 = no limit)
	if opts.Limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", opts.Limit))
	}

	fullURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := defaultHTTPClient.Do(req)
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

	// Register/get project
	project, err := api.RegisterProject(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to register project: %w", err)
	}
	projectID := project.ID

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

	resp, err := defaultHTTPClient.Do(req)
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

	// Register/get project
	project, err := api.RegisterProject(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to register project: %w", err)
	}
	projectID := project.ID

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

	resp, err := defaultHTTPClient.Do(req)
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

// createPrompt creates a prompt function (not a scorer) for use in evals/tasks
func createPrompt(projectName, name, slug, description string, promptData map[string]any) (string, error) {
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return "", fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Register/get project
	project, err := api.RegisterProject(projectName)
	if err != nil {
		return "", fmt.Errorf("failed to register project: %w", err)
	}
	projectID := project.ID

	// Build the request payload for a prompt (not scorer)
	// Note: function_type should be null/omitted for prompts, not "prompt"
	payload := map[string]any{
		"project_id": projectID,
		"name":       name,
		"slug":       slug,
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

	resp, err := defaultHTTPClient.Do(req)
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

type functionScorer[I, R any] struct {
	name       string
	functionID string
	client     *http.Client
}

// newFunctionScorer creates a new function scorer using the shared HTTP client
func newFunctionScorer[I, R any](name, functionID string) *functionScorer[I, R] {
	return &functionScorer[I, R]{
		name:       name,
		functionID: functionID,
		client:     defaultHTTPClient,
	}
}

func (f *functionScorer[I, R]) Name() string {
	return f.name
}

// GetTask creates a Task from a Braintrust function (prompt/tool).
// The server handles templating the prompt with input variables for each eval case.
// This is equivalent to initFunction() in TypeScript and init_function() in Python.
//
// The function will automatically unmarshal JSON responses into the specified return type R.
//
// Example usage:
//
//	eval.Eval(ctx, eval.Options{
//	    ProjectName: "my-project",
//	    Data:        dataset,
//	    Task: functions.GetTask[string, string](functions.Opts{
//	        Slug: "my-prompt",
//	    }),
//	    Scorers: scorers,
//	})
func GetTask[I, R any](opts Opts) eval.Task[I, R] {
	return func(ctx context.Context, input I) (R, error) {
		result, err := invoke(ctx, invokeOptions{
			Project:     opts.Project,
			ProjectID:   opts.ProjectID,
			Slug:        opts.Slug,
			FunctionID:  opts.FunctionID,
			Version:     opts.Version,
			Environment: opts.Environment,
			Input:       input,
		})
		if err != nil {
			var zero R
			return zero, err
		}

		// Try direct type assertion first (works for simple types like string, int, etc.)
		typedResult, ok := result.(R)
		if ok {
			return typedResult, nil
		}

		// For complex types (structs) or type mismatches, we need to convert via JSON
		var zero R

		// If result is a string, it might be a JSON string that needs parsing
		// This handles cases where the LLM returns JSON as a string
		if resultStr, ok := result.(string); ok {
			// Try to unmarshal the string as JSON
			if err := json.Unmarshal([]byte(resultStr), &zero); err != nil {
				// If unmarshaling fails and R is string type (including custom string types),
				// return the string as-is. This handles cases where GetTask[string, string]
				// or GetTask[CustomString, CustomString] receives a plain string.
				// Use reflection to check if the underlying type is string to support type aliases.
				if reflect.TypeOf(zero).Kind() == reflect.String {
					// Use reflection to convert the string to the target type (handles custom string types)
					resultValue := reflect.ValueOf(resultStr)
					typedValue := resultValue.Convert(reflect.TypeOf(zero))
					typedResult, ok := typedValue.Interface().(R)
					if !ok {
						return zero, fmt.Errorf("failed to convert string to type %T", zero)
					}
					return typedResult, nil
				}
				return zero, fmt.Errorf("failed to unmarshal JSON string to type %T: %w", zero, err)
			}
			return zero, nil
		}

		// Otherwise, result is likely a map[string]any from JSON parsing
		// Marshal and unmarshal to convert to the target type
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return zero, fmt.Errorf("failed to marshal result to JSON: %w", err)
		}

		if err := json.Unmarshal(jsonBytes, &zero); err != nil {
			return zero, fmt.Errorf("failed to unmarshal result to type %T: %w", zero, err)
		}

		return zero, nil
	}
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
