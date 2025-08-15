package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
)

// GetDatasetByID returns Cases for the dataset with the given ID.
func GetDatasetByID[I, R any](datasetID string) (Cases[I, R], error) {
	if datasetID == "" {
		return nil, fmt.Errorf("dataset ID is required")
	}

	return &datasetIterator[I, R]{
		dataset: api.NewDataset(datasetID),
	}, nil
}

// GetDataset returns the most recent dataset with the given project name and dataset name.
func GetDataset[I, R any](projectName, datasetName string) (Cases[I, R], error) {
	opts := DatasetOpts{
		ProjectName: projectName,
		DatasetName: datasetName,
		Limit:       1,
	}
	return QueryDataset[I, R](opts)
}

// QueryDataset returns Cases for datasets matching the given options.
func QueryDataset[I, R any](opts DatasetOpts) (Cases[I, R], error) {
	datasets, err := queryDatasets(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets: %w", err)
	}

	if len(datasets) == 0 {
		return nil, fmt.Errorf("no datasets found matching the criteria")
	}

	// Return Cases for the first (most recent) dataset
	return &datasetIterator[I, R]{
		dataset: api.NewDataset(datasets[0].ID),
	}, nil
}

// DatasetOpts provides flexible options for querying Braintrust datasets
type DatasetOpts struct {
	// Project identity (either/or)
	ProjectName string // Filter by project name
	ProjectID   string // Filter by specific project ID

	// Dataset identity (either/or)
	DatasetName string // Filter by dataset name
	DatasetID   string // Use specific dataset ID directly

	// Query modifiers
	Version string // Specific dataset version
	Limit   int    // Max results (default: no limit)
}

// DatasetInfo represents a Braintrust dataset.
type DatasetInfo struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// queryDatasets queries the Braintrust API for datasets matching the options
func queryDatasets(opts DatasetOpts) ([]DatasetInfo, error) {
	// If dataset ID is provided directly, create a dataset entry
	if opts.DatasetID != "" {
		return []DatasetInfo{{
			ID:   opts.DatasetID,
			Name: opts.DatasetID, // Use ID as name fallback
		}}, nil
	}

	// Otherwise query the API
	config := braintrust.GetConfig()
	if config.APIKey == "" {
		return nil, fmt.Errorf("BRAINTRUST_API_KEY is required")
	}

	// Build the URL with query parameters
	baseURL := fmt.Sprintf("%s/v1/dataset", config.APIURL)
	params := url.Values{}

	if opts.ProjectName != "" {
		params.Add("project_name", opts.ProjectName)
	}
	if opts.ProjectID != "" {
		params.Add("project_id", opts.ProjectID)
	}
	if opts.DatasetName != "" {
		params.Add("dataset_name", opts.DatasetName)
	}
	if opts.Version != "" {
		params.Add("version", opts.Version)
	}

	// Add limit if specified
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
		Objects []DatasetInfo `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Objects, nil
}

type datasetIterator[InputType, ExpectedType any] struct {
	dataset *api.Dataset
}

func (s *datasetIterator[InputType, ExpectedType]) Next() (Case[InputType, ExpectedType], error) {
	var fullEvent struct {
		Input    InputType    `json:"input"`
		Expected ExpectedType `json:"expected"`
	}

	err := s.dataset.NextAs(&fullEvent)
	if err != nil {
		var zero Case[InputType, ExpectedType]
		return zero, err
	}

	return Case[InputType, ExpectedType]{
		Input:    fullEvent.Input,
		Expected: fullEvent.Expected,
	}, nil
}
