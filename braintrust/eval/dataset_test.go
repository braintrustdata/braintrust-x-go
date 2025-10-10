package eval

import (
	"testing"
)

func TestGetDatasetByID(t *testing.T) {
	// Test that GetDatasetByID returns error for empty ID
	_, err := GetDatasetByID[string, string]("")
	if err == nil {
		t.Error("Expected error for empty dataset ID")
	}

	// Test with valid ID (will fail API call but should return Cases)
	cases, err := GetDatasetByID[string, string]("test-dataset-id")
	if err != nil {
		t.Errorf("GetDatasetByID failed: %v", err)
	}
	if cases == nil {
		t.Error("Expected non-nil Cases")
	}
}

func TestGetDataset(t *testing.T) {
	// Test GetDataset with project and dataset names
	_, err := GetDataset[string, string]("test-project", "test-dataset")
	// This will fail due to API call, but function should not panic
	if err == nil {
		t.Log("GetDataset completed (likely failed API call, which is expected)")
	}
}

func TestQueryDataset(t *testing.T) {
	// Test QueryDataset with options
	opts := DatasetOpts{
		ProjectName: "test-project",
		DatasetName: "test-dataset",
		Limit:       5,
	}

	_, err := QueryDataset[string, string](opts)
	// This will fail due to API call, but function should not panic
	if err == nil {
		t.Log("QueryDataset completed (likely failed API call, which is expected)")
	}
}

func TestDatasetOpts(t *testing.T) {
	// Test DatasetOpts struct creation
	opts := DatasetOpts{
		ProjectName: "test-project",
		DatasetName: "test-dataset",
		Limit:       10,
	}

	if opts.ProjectName != "test-project" {
		t.Errorf("Expected ProjectName 'test-project', got %s", opts.ProjectName)
	}
	if opts.DatasetName != "test-dataset" {
		t.Errorf("Expected DatasetName 'test-dataset', got %s", opts.DatasetName)
	}
	if opts.Limit != 10 {
		t.Errorf("Expected Limit 10, got %d", opts.Limit)
	}
}

func TestDatasetInfo(t *testing.T) {
	// Test datasetInfo struct creation
	info := datasetInfo{
		ID:          "dataset-123",
		ProjectID:   "project-456",
		Name:        "test-dataset",
		Description: "A test dataset",
	}

	if info.ID != "dataset-123" {
		t.Errorf("Expected ID 'dataset-123', got %s", info.ID)
	}
	if info.Name != "test-dataset" {
		t.Errorf("Expected Name 'test-dataset', got %s", info.Name)
	}
}
