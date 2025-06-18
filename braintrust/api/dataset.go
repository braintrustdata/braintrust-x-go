package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/braintrust/braintrust-x-go/braintrust"
)

// DatasetEvent represents a single event/record in a dataset
type DatasetEvent struct {
	ID       string      `json:"id,omitempty"`
	Input    interface{} `json:"input"`
	Expected interface{} `json:"expected,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// DatasetRequest represents the request payload for creating a dataset
type DatasetRequest struct {
	ProjectID   string                 `json:"project_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DatasetInfo represents a dataset from the API
type DatasetInfo struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DatasetInsertRequest represents the request payload for inserting events
type DatasetInsertRequest struct {
	Events []DatasetEvent `json:"events"`
}

// DatasetFetchRequest represents the request payload for fetching events
type DatasetFetchRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

// DatasetFetchResponse represents the response from fetching events
type DatasetFetchResponse struct {
	Events []json.RawMessage `json:"events"`
	Cursor string            `json:"cursor,omitempty"`
}

// FetchDatasetEvents retrieves events from a dataset
func FetchDatasetEvents(datasetID string, req DatasetFetchRequest) (*DatasetFetchResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	baseURL, err := url.Parse(config.APIURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %w", err)
	}

	// MANU_COMMENT: We should probably use `/btql` here so that we can inject
	// filters and other stuff. We kind of want to get away from the /fetch
	// endpoints because they are very limited compared to btql.
	endpoint, err := url.Parse(fmt.Sprintf("/v1/dataset/%s/fetch", datasetID))
	if err != nil {
		return nil, fmt.Errorf("error parsing endpoint: %w", err)
	}

	fullURL := baseURL.ResolveReference(endpoint)

	httpReq, err := http.NewRequest("POST", fullURL.String(), bytes.NewBuffer(jsonData))
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

	var result DatasetFetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}

// MANU_COMMENT: Should we have the GetOrCreateDataset method too (or instead) which also handles project registration?

// CreateDataset creates a new dataset via the Braintrust API
func CreateDataset(req DatasetRequest) (*DatasetInfo, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	baseURL, err := url.Parse(config.APIURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %w", err)
	}

	endpoint, err := url.Parse("/v1/dataset")
	if err != nil {
		return nil, fmt.Errorf("error parsing endpoint: %w", err)
	}

	fullURL := baseURL.ResolveReference(endpoint)

	httpReq, err := http.NewRequest("POST", fullURL.String(), bytes.NewBuffer(jsonData))
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

	var result DatasetInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}

// InsertDatasetEvents inserts events into a dataset
func InsertDatasetEvents(datasetID string, events []DatasetEvent) error {
	req := DatasetInsertRequest{
		Events: events,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	baseURL, err := url.Parse(config.APIURL)
	if err != nil {
		return fmt.Errorf("error parsing base URL: %w", err)
	}

	// MANU_COMMENT: I think for these sorts of things we should use the async
	// background logger similar to what we do in python/TS. We don't want every
	// insert to be a synchronous web request.
	endpoint, err := url.Parse(fmt.Sprintf("/v1/dataset/%s/insert", datasetID))
	if err != nil {
		return fmt.Errorf("error parsing endpoint: %w", err)
	}

	fullURL := baseURL.ResolveReference(endpoint)

	httpReq, err := http.NewRequest("POST", fullURL.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Dataset handles fetching raw DatasetEvents from the Braintrust API with pagination
type Dataset struct {
	DatasetID string
	Events    []json.RawMessage
	Index     int
	Cursor    string
	Exhausted bool
}

// NewDataset creates a new Dataset that fetches data from the given dataset ID
func NewDataset(datasetID string) *Dataset {
	return &Dataset{
		DatasetID: datasetID,
		Events:    nil,
		Index:     0,
		Cursor:    "",
		Exhausted: false,
	}
}

// Next returns the next DatasetEvent, fetching more data as needed
func (d *Dataset) Next() (DatasetEvent, error) {
	// If we've consumed all events in the current batch and haven't exhausted the dataset, fetch more
	if d.Index >= len(d.Events) && !d.Exhausted {
		err := d.fetchNextBatch()
		if err != nil {
			return DatasetEvent{}, err
		}
	}

	// If we still don't have any events, we're done
	if d.Index >= len(d.Events) {
		return DatasetEvent{}, io.EOF
	}

	// Unmarshal the raw message into a DatasetEvent
	var event DatasetEvent
	err := json.Unmarshal(d.Events[d.Index], &event)
	if err != nil {
		return DatasetEvent{}, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	d.Index++
	return event, nil
}

// fetchNextBatch retrieves the next batch of events from the Braintrust API
func (d *Dataset) fetchNextBatch() error {
	req := DatasetFetchRequest{
		Limit:  100, // Fetch 100 records at a time
		Cursor: d.Cursor,
	}

	resp, err := FetchDatasetEvents(d.DatasetID, req)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset events: %w", err)
	}

	d.Events = resp.Events
	d.Index = 0
	d.Cursor = resp.Cursor

	// If no cursor is returned or no events, we've exhausted the dataset
	if resp.Cursor == "" || len(resp.Events) == 0 {
		d.Exhausted = true
	}

	return nil
}

// NextAs unmarshals the next event into the given struct type
func (d *Dataset) NextAs(target interface{}) error {
	// If we've consumed all events in the current batch and haven't exhausted the dataset, fetch more
	if d.Index >= len(d.Events) && !d.Exhausted {
		err := d.fetchNextBatch()
		if err != nil {
			return err
		}
	}

	// If we still don't have any events, we're done
	if d.Index >= len(d.Events) {
		return io.EOF
	}

	// Unmarshal the raw message into the target struct
	err := json.Unmarshal(d.Events[d.Index], target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	d.Index++
	return nil
}
