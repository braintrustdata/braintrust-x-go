package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"

	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/oteltest"
	"github.com/braintrustdata/braintrust-x-go/internal/tests"
)

// testInput and testOutput are simple types for testing
type testInput struct {
	Value string `json:"value"`
}

type testOutput struct {
	Result string `json:"result"`
}

// unitTestEval wraps eval with testing utilities
type unitTestEval[I, R any] struct {
	eval     *eval[I, R]
	exporter *oteltest.Exporter
}

// newUnitTestEval creates a fully configured eval for unit testing with fake data.
// It generates its own fake session, config, tracer, experiment/project IDs, etc.
func newUnitTestEval[I, R any](t *testing.T, cases Cases[I, R], task Task[I, R], scorers []Scorer[I, R], parallelism int) *unitTestEval[I, R] {
	t.Helper()

	// Create test tracer and exporter using oteltest
	tracer, exporter := oteltest.Setup(t)

	// Create fake session with test data
	session := tests.NewSession(t)

	// Create test config
	cfg := &config.Config{
		AppURL:             "https://test.braintrust.dev",
		OrgName:            "test-org",
		DefaultProjectName: "test-project",
	}

	// Create eval with fake IDs
	e := testNewEval(
		cfg,
		session,
		tracer,
		"exp-12345678",    // fake experiment ID
		"test-experiment", // fake experiment name
		"proj-87654321",   // fake project ID
		"test-project",    // fake project name
		cases,
		task,
		scorers,
		parallelism,
	)

	return &unitTestEval[I, R]{
		eval:     e,
		exporter: exporter,
	}
}

// simpleScorer is a test scorer that returns a fixed score
type simpleScorer struct {
	name  string
	score float64
	meta  map[string]interface{}
	err   error
}

func (s *simpleScorer) Name() string {
	return s.name
}

func (s *simpleScorer) Run(ctx context.Context, input testInput, expected testOutput, result testOutput, meta Metadata) ([]Score, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []Score{{
		Name:     s.name,
		Score:    s.score,
		Metadata: s.meta,
	}}, nil
}

func TestNewEval_Success(t *testing.T) {
	t.Parallel()

	// Create test cases with tags and metadata
	cases := NewCases([]Case[testInput, testOutput]{
		{
			Input:    testInput{Value: "test1"},
			Expected: testOutput{Result: "expected1"},
			Tags:     []string{"tag1", "tag2"},
			Metadata: map[string]interface{}{"key": "value"},
		},
		{
			Input:    testInput{Value: "test2"},
			Expected: testOutput{Result: "expected2"},
		},
	})

	// Create test task
	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "output-" + input.Value}, nil
	}

	// Create test scorers
	scorers := []Scorer[testInput, testOutput]{
		&simpleScorer{name: "accuracy", score: 0.95, meta: map[string]interface{}{"note": "good"}},
	}

	// Create eval
	ute := newUnitTestEval(t, cases, task, scorers, 1)

	// Verify eval was created correctly
	assert.NotNil(t, ute.eval)
	assert.Equal(t, "exp-12345678", ute.eval.experimentID)
	assert.Equal(t, "test-experiment", ute.eval.experimentName)
	assert.Equal(t, "proj-87654321", ute.eval.projectID)
	assert.Equal(t, "test-project", ute.eval.projectName)
	assert.Equal(t, 1, ute.eval.goroutines)
	assert.NotNil(t, ute.eval.tracer)
	assert.NotNil(t, ute.eval.startSpanOpt)

	// Verify permalink generation
	permalink := ute.eval.permalink()
	assert.Equal(t, "https://test.braintrust.dev/app/test-org/object?object_type=experiment&object_id=exp-12345678", permalink)

	// Run the eval and verify span structure
	ctx := context.Background()
	result, err := ute.eval.run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify all spans were created with correct structure
	// Spans are in completion order: task, score, eval for each case
	spans := ute.exporter.Flush()
	require.Len(t, spans, 6) // 2 cases * (task + score + eval) = 6 spans

	// First case spans (in completion order: task, score, eval)
	spans[0].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      map[string]any{"value": "test1"},
			"braintrust.expected":        map[string]any{"result": "expected1"},
			"braintrust.output_json":     map[string]any{"result": "output-test1"},
			"braintrust.span_attributes": map[string]any{"type": "task"},
		},
	})

	spans[1].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
		},
		JSONAttrs: map[string]any{
			"braintrust.span_attributes": map[string]any{"type": "score"},
			"braintrust.scores":          map[string]any{"accuracy": 0.95},
			"braintrust.metadata":        map[string]any{"note": "good"},
			"braintrust.output":          map[string]any{"score": 0.95},
		},
	})

	spans[2].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
			"braintrust.tags":   []string{"tag1", "tag2"},
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      map[string]any{"value": "test1"},
			"braintrust.output_json":     map[string]any{"result": "output-test1"},
			"braintrust.expected":        map[string]any{"result": "expected1"},
			"braintrust.metadata":        map[string]any{"key": "value"},
			"braintrust.span_attributes": map[string]any{"type": "eval"},
		},
	})

	// Second case spans (no tags or metadata)
	spans[3].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      map[string]any{"value": "test2"},
			"braintrust.expected":        map[string]any{"result": "expected2"},
			"braintrust.output_json":     map[string]any{"result": "output-test2"},
			"braintrust.span_attributes": map[string]any{"type": "task"},
		},
	})

	spans[4].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
		},
		JSONAttrs: map[string]any{
			"braintrust.span_attributes": map[string]any{"type": "score"},
			"braintrust.scores":          map[string]any{"accuracy": 0.95},
			"braintrust.metadata":        map[string]any{"note": "good"},
			"braintrust.output":          map[string]any{"score": 0.95},
		},
	})

	spans[5].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:exp-12345678",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      map[string]any{"value": "test2"},
			"braintrust.output_json":     map[string]any{"result": "output-test2"},
			"braintrust.expected":        map[string]any{"result": "expected2"},
			"braintrust.span_attributes": map[string]any{"type": "eval"},
		},
	})
}

func TestNewEval_Parallelism(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test"}},
	})
	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "ok"}, nil
	}

	ute := newUnitTestEval(t, cases, task, nil, 4)
	assert.Equal(t, 4, ute.eval.goroutines)
}

func TestNewEval_DefaultParallelism(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test"}},
	})
	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "ok"}, nil
	}

	// Test with 0
	ute := newUnitTestEval(t, cases, task, nil, 0)
	assert.Equal(t, 1, ute.eval.goroutines)

	// Test with negative
	ute2 := newUnitTestEval(t, cases, task, nil, -5)
	assert.Equal(t, 1, ute2.eval.goroutines)
}

func TestEval_Run_TaskError(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test1"}},
		{Input: testInput{Value: "error"}},
		{Input: testInput{Value: "test2"}},
	})

	// Task that fails on specific input
	taskErr := errors.New("task failed")
	task := func(ctx context.Context, input testInput) (testOutput, error) {
		if input.Value == "error" {
			return testOutput{}, taskErr
		}
		return testOutput{Result: "ok-" + input.Value}, nil
	}

	ute := newUnitTestEval(t, cases, task, nil, 1)

	ctx := context.Background()
	result, err := ute.eval.run(ctx)

	// Should return error but continue processing other cases
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task failed")
	assert.NotNil(t, result)

	// Verify spans (in completion order: task, score, eval per case)
	// Score span is only created when task succeeds
	spans := ute.exporter.Flush()
	require.Len(t, spans, 8) // First: 3 spans, Second: 2 spans (no score), Third: 3 spans

	// First case succeeds (task, score, eval)
	spans[0].AssertNameIs("task")
	spans[1].AssertNameIs("score")
	spans[2].AssertNameIs("eval")

	// Second case fails (task with error, then eval with error - NO score span)
	spans[3].AssertNameIs("task")
	assert.Equal(t, codes.Error, spans[3].Status().Code)
	events := spans[3].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "exception", events[0].Name)

	spans[4].AssertNameIs("eval")
	assert.Equal(t, codes.Error, spans[4].Status().Code)

	// Third case succeeds (task, score, eval)
	spans[5].AssertNameIs("task")
	spans[6].AssertNameIs("score")
	spans[7].AssertNameIs("eval")
}

func TestEval_Run_ScorerError(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test1"}},
		{Input: testInput{Value: "test2"}},
	})

	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "ok"}, nil
	}

	// Scorers: one succeeds, one fails
	scorerErr := errors.New("scorer failed")
	scorers := []Scorer[testInput, testOutput]{
		&simpleScorer{name: "good-scorer", score: 0.8},
		&simpleScorer{name: "bad-scorer", err: scorerErr},
		&simpleScorer{name: "another-good-scorer", score: 0.9},
	}

	ute := newUnitTestEval(t, cases, task, scorers, 1)

	ctx := context.Background()
	result, err := ute.eval.run(ctx)

	// Should return error but continue processing
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scorer failed")
	assert.NotNil(t, result)

	// Verify spans (in completion order: task, score, eval per case)
	spans := ute.exporter.Flush()
	require.Len(t, spans, 6) // 2 cases * (task + score + eval) = 6 spans

	// First case (task succeeds, scorer partially fails, eval fails)
	spans[0].AssertNameIs("task")

	// Score span should have error status and only successful scores recorded
	spans[1].AssertNameIs("score")
	assert.Equal(t, codes.Error, spans[1].Status().Code)
	spans[1].AssertJSONAttrEquals("braintrust.scores", map[string]any{
		"good-scorer":         0.8,
		"another-good-scorer": 0.9,
	})
	spans[1].AssertJSONAttrEquals("braintrust.output", map[string]any{
		"good-scorer":         map[string]any{"score": 0.8},
		"another-good-scorer": map[string]any{"score": 0.9},
	})
	events := spans[1].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "exception", events[0].Name)

	spans[2].AssertNameIs("eval")
	assert.Equal(t, codes.Error, spans[2].Status().Code)

	// Second case (same pattern)
	spans[3].AssertNameIs("task")
	spans[4].AssertNameIs("score")
	assert.Equal(t, codes.Error, spans[4].Status().Code)
	spans[5].AssertNameIs("eval")
	assert.Equal(t, codes.Error, spans[5].Status().Code)
}

func TestEval_Run_PrintsSummary(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test1"}},
		{Input: testInput{Value: "test2"}},
	})

	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "ok"}, nil
	}

	ute := newUnitTestEval(t, cases, task, nil, 1)
	// Set quiet to false to enable summary printing
	ute.eval.quiet = false

	// Capture output by providing a custom writer
	// For now, just verify the result has expected fields for String()
	ctx := context.Background()
	result, err := ute.eval.run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify String() produces expected output
	summary := result.String()
	assert.Contains(t, summary, "=== Experiment: test-experiment ===")
	assert.Contains(t, summary, "Name: test-experiment")
	assert.Contains(t, summary, "Project: test-project")
	assert.Contains(t, summary, "Duration:")
	assert.Contains(t, summary, "Link: https://test.braintrust.dev/app/test-org/object?object_type=experiment&object_id=exp-12345678")
}

func TestEval_Run_QuietSuppressesSummary(t *testing.T) {
	t.Parallel()

	cases := NewCases([]Case[testInput, testOutput]{
		{Input: testInput{Value: "test1"}},
	})

	task := func(ctx context.Context, input testInput) (testOutput, error) {
		return testOutput{Result: "ok"}, nil
	}

	ute := newUnitTestEval(t, cases, task, nil, 1)
	// Ensure quiet is true (should be default from testNewEval)
	assert.True(t, ute.eval.quiet, "quiet should be true by default in test helper")

	ctx := context.Background()
	result, err := ute.eval.run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// When quiet is true, summary should not be printed
	// We can't easily capture stdout in a test, but we verify:
	// 1. quiet is set correctly
	// 2. The result object is still valid
	summary := result.String()
	assert.NotEmpty(t, summary, "result should still have a valid String() representation")
}
