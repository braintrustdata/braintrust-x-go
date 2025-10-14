package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	attr "go.opentelemetry.io/otel/attribute"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
)

// defaultHTTPClient is a shared HTTP client with reasonable timeouts.
// It is safe for concurrent use by multiple goroutines.
var defaultHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// invokeOptions provides options for invoking a Braintrust function.
type invokeOptions struct {
	// Project identity (either/or)
	Project   string
	ProjectID string

	// Function identity (either/or)
	Slug       string
	FunctionID string

	// Query modifiers
	Version     string
	Environment string

	// Input data to pass to the function
	Input any
}

// invoke calls a Braintrust function with the given input.
// The server handles all templating and LLM execution.
// Returns the output from the function invocation.
func invoke(ctx context.Context, opts invokeOptions) (any, error) {
	// Validate that we have enough information to identify the function
	if opts.FunctionID == "" && opts.Slug == "" {
		return nil, fmt.Errorf("either FunctionID or Slug must be specified")
	}
	if opts.FunctionID == "" && opts.Project == "" && opts.ProjectID == "" {
		return nil, fmt.Errorf("either FunctionID or Project/ProjectID must be specified")
	}

	// Create a span for the function invocation
	tracer := otel.GetTracerProvider().Tracer("braintrust.functions")
	spanName := opts.Slug
	if spanName == "" {
		spanName = opts.FunctionID
	}
	if spanName == "" {
		spanName = "unknown"
	}
	ctx, span := tracer.Start(ctx, fmt.Sprintf("function: %s", spanName))
	defer span.End()

	// Set Braintrust span attributes to mark this as a function invocation
	spanAttrs := map[string]any{"type": "function"}
	spanAttrsJSON, _ := json.Marshal(spanAttrs)
	span.SetAttributes(attr.String("braintrust.span_attributes", string(spanAttrsJSON)))

	// Set metadata about the function invocation
	metadata := map[string]any{}
	if opts.Slug != "" {
		metadata["slug"] = opts.Slug
	}
	if opts.Version != "" {
		metadata["version"] = opts.Version
	}
	if opts.Environment != "" {
		metadata["environment"] = opts.Environment
	}
	if opts.Project != "" {
		metadata["project_name"] = opts.Project
	}
	if len(metadata) > 0 {
		metadataJSON, _ := json.Marshal(metadata)
		span.SetAttributes(attr.String("braintrust.metadata", string(metadataJSON)))
	}

	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return nil, fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Resolve function ID if not provided directly
	functionID := opts.FunctionID
	if functionID == "" {
		// Query for function by project+slug
		functions, err := queryFunctions(ctx, Opts{
			Project:     opts.Project,
			ProjectID:   opts.ProjectID,
			Slug:        opts.Slug,
			Version:     opts.Version,
			Environment: opts.Environment,
			Limit:       1,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query function: %w", err)
		}
		if len(functions) == 0 {
			projectInfo := opts.Project
			if projectInfo == "" {
				projectInfo = opts.ProjectID
			}
			return nil, fmt.Errorf("function not found: project=%s slug=%s", projectInfo, opts.Slug)
		}
		functionID = functions[0].ID
	}

	// Build request payload
	payload := map[string]any{
		"input": opts.Input,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/v1/function/%s/invoke", config.APIURL, functionID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	// Execute request
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke function: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("function invocation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - try as object first, then as raw value
	var response map[string]any
	if err := json.Unmarshal(body, &response); err == nil {
		// Response is an object, extract output field
		output, ok := response["output"]
		if !ok {
			return nil, fmt.Errorf("response missing 'output' field")
		}
		return output, nil
	}

	// Response is not an object, try parsing as raw JSON value (string, number, etc.)
	var output any
	if err := json.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return output, nil
}
