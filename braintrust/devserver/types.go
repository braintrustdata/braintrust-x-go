package devserver

import (
	"context"

	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

// RemoteEval represents an evaluator that can be executed remotely from the Braintrust web interface.
// It uses generics to provide type safety for users while the server handles JSON serialization internally.
type RemoteEval[I, R any] struct {
	// Name is the unique identifier for this evaluator
	Name string

	// ProjectName is the Braintrust project this evaluator belongs to
	ProjectName string

	// Task is the evaluation task that processes inputs and returns outputs
	Task eval.Task[I, R]

	// Scorers are the scoring functions that evaluate the task outputs
	Scorers []eval.Scorer[I, R]
}

// registeredEval is the internal type-erased representation of a RemoteEval.
// It handles JSON marshaling/unmarshaling so users don't have to work with interface{}.
type registeredEval struct {
	name        string
	projectName string

	// Type-erased task function that accepts and returns interface{}
	// The Register method wraps the typed task to handle JSON conversion
	task func(context.Context, interface{}) (interface{}, error)

	// Type-erased scorer functions
	scorers []func(context.Context, interface{}, interface{}, interface{}, eval.Metadata) (eval.Scores, error)

	// Scorer names extracted from the original Scorer.Name() methods
	scorerNames []string
}

// evalRequest represents the request body for POST /eval
type evalRequest struct {
	Name           string                 `json:"name"`            // Required: evaluator name
	Parameters     map[string]interface{} `json:"parameters"`      // Optional: parameter overrides
	Data           dataSpec               `json:"data"`            // Required: dataset specification
	ExperimentName string                 `json:"experiment_name"` // Optional: experiment name override
	ProjectID      string                 `json:"project_id"`      // Optional: project ID override
	Stream         bool                   `json:"stream"`          // Optional: enable SSE streaming
}

// dataSpec specifies where to get evaluation data
type dataSpec struct {
	// Option 1: By project/dataset name
	ProjectName string `json:"project_name,omitempty"`
	DatasetName string `json:"dataset_name,omitempty"`

	// Option 2: By dataset ID
	DatasetID string `json:"dataset_id,omitempty"`

	// Option 3: Inline data
	Data []map[string]interface{} `json:"data,omitempty"`
}

// evalResponse represents the response from POST /eval (non-streaming)
type evalResponse struct {
	ExperimentName string                 `json:"experimentName"`
	ProjectName    string                 `json:"projectName"`
	ProjectID      string                 `json:"projectId"`
	ExperimentID   string                 `json:"experimentId"`
	ExperimentURL  string                 `json:"experimentUrl"`
	ProjectURL     string                 `json:"projectUrl"`
	Scores         map[string]interface{} `json:"scores"` // TODO: populate with actual scores
}
