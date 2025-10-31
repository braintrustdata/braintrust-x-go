// Package eval provides evaluation functionality for the Braintrust SDK.
// This package is designed to work with the client-based architecture and
// does not rely on global state.
package eval

import (
	"context"
	"fmt"
	"time"
)

// key contains the data needed to uniquely identify and reference an eval.
// This is used internally by Result and is not exported.
type key struct {
	experimentID string
	name         string
	projectID    string
	projectName  string
}

// Opts defines the options for running an evaluation.
// I is the input type and R is the result/output type.
type Opts[I, R any] struct {
	// Experiment is the name of the experiment to create or use.
	// Required.
	Experiment string

	// Cases is an iterator over the test cases to evaluate.
	// Required.
	Cases Cases[I, R]

	// Task is the function to evaluate for each case.
	// It receives the input and should return the output.
	// Required.
	Task Task[I, R]

	// Scorers are the scoring functions to apply to each case result.
	// Optional. If empty, no scoring is performed.
	Scorers []Scorer[I, R]

	// Tags are labels to attach to the experiment.
	// Optional.
	Tags []string

	// Metadata is additional metadata to attach to the experiment.
	// Optional.
	Metadata map[string]interface{}

	// Update controls whether to update an existing experiment or fail if it exists.
	// Optional. Defaults to false.
	Update bool

	// Parallelism controls the number of goroutines to use for parallel execution.
	// Optional. Defaults to 1 (sequential execution).
	// Set to a value > 1 to enable parallel case evaluation.
	Parallelism int

	// Quiet controls whether to suppress printing the result summary.
	// Optional. Defaults to false (summary is printed).
	// Set to true to suppress output.
	Quiet bool
}

// Case represents a single test case in an evaluation.
type Case[I, R any] struct {
	// Input is the input to the task function.
	Input I

	// Expected is the expected output (for scoring).
	// Optional.
	Expected R

	// Tags are labels to attach to this case.
	// Optional.
	Tags []string

	// Metadata is additional metadata for this case.
	// Optional.
	Metadata map[string]interface{}
}

// Cases is an iterator interface for test cases.
// This allows lazy loading of cases without requiring them all in memory.
// Implementations must return io.EOF when iteration is complete.
type Cases[I, R any] interface {
	// Next returns the next case, or io.EOF if there are no more cases.
	Next() (Case[I, R], error)
}

// Task is a function that processes an input and returns an output.
// This is the function being evaluated.
type Task[I, R any] func(ctx context.Context, input I) (R, error)

// Metadata is a map of strings to a JSON-encodable value. It is used to store arbitrary metadata about a case.
type Metadata map[string]any

// Scorer is an interface for scoring the output of a task.
type Scorer[I, R any] interface {
	// Name returns the name of this scorer.
	Name() string
	// Run evaluates the output against the expected result.
	// It returns one or more Score results.
	Run(ctx context.Context, input I, expected R, result R, meta Metadata) ([]Score, error)
}

// Score represents a single score result.
type Score struct {
	// Name is the name of the score (e.g., "accuracy", "exact_match").
	Name string

	// Score is the numeric score value.
	Score float64

	// Metadata is optional additional metadata for this score.
	Metadata map[string]interface{}
}

// Result contains the results of an evaluation.
type Result struct {
	key       key
	err       error
	elapsed   time.Duration
	permalink string
	// TODO: Will be populated with span data, scores, errors, etc. in future iterations
}

// newResult creates a new Result with the given parameters.
func newResult(k key, err error, permalink string, elapsed time.Duration) *Result {
	return &Result{
		err:       err,
		permalink: permalink,
		elapsed:   elapsed,
		key:       k,
	}
}

// Permalink returns link to this eval in the Braintrust UI.
func (r *Result) Permalink() (string, error) {
	return r.permalink, nil
}

// Error returns the error from running the eval.
func (r *Result) Error() error {
	return r.err
}

// Name returns the experiment name.
func (r *Result) Name() string {
	return r.key.name
}

// ID returns the experiment ID.
func (r *Result) ID() string {
	return r.key.experimentID
}

// String returns a string representaton of the result for printing on the console.
//
// The format it prints will change and shouldn't be relied on for programmatic use.
func (r *Result) String() string {
	link, linkErr := r.Permalink()

	projectDisplay := r.key.projectName
	if projectDisplay == "" {
		projectDisplay = r.key.projectID
	}

	lines := []string{
		"",
		fmt.Sprintf("=== Experiment: %s ===", r.key.name),
		fmt.Sprintf("Name: %s", r.key.name),
		fmt.Sprintf("Project: %s", projectDisplay),
		fmt.Sprintf("Duration: %.1fs", r.elapsed.Seconds()),
		fmt.Sprintf("Link: %s", link),
	}
	if linkErr != nil {
		lines = append(lines, fmt.Sprintf("Warning: Failed to generate permalink: %v", linkErr))
	}

	// Error details if present
	if r.err != nil {
		lines = append(lines, "Errors:")
		lines = append(lines, "  "+r.err.Error())
	}

	lines = append(lines, "")

	// Join all lines
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
