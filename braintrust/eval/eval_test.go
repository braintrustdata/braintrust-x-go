package eval

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
	"github.com/braintrust/braintrust-x-go/braintrust/trace"
)

var (
	scoreType = map[string]string{"type": "score"}
	evalType  = map[string]string{"type": "eval"}
	taskType  = map[string]string{"type": "task"}
)

func TestEval_TaskErrors(t *testing.T) {
	// a test that verifies we properly handle evals where some
	// tasks pass and some have errors.

	assert, require := assert.New(t), require.New(t)
	assert.NotNil(require)

	_, exporter := oteltest.Setup(t)

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 2, Expected: 4},
	}

	task := func(ctx context.Context, x int) (int, error) {
		if x%2 == 0 {
			return x, nil
		}
		return 0, errors.New("oops")
	}

	scorers := []Scorer[int, int]{
		autoevals.NewEquals[int, int](),
	}

	eval := New("123", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run()
	timeRange := timer.Tick()

	assert.ErrorIs(err, ErrTaskRun)
	assert.Contains(err.Error(), "oops")

	spans := exporter.Flush()
	// With errors, we don't get all since not everything is scored
	assert.Equal(5, len(spans))

	for _, span := range spans {
		span.AssertInTimeRange(timeRange)
	}

	// First span is the failed task - it has an exception event
	spans[0].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.parent":   "experiment_id:123",
			"braintrust.expected": "2",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      1,
			"braintrust.span_attributes": taskType,
		},
		StatusCode:        codes.Error,
		StatusDescription: "task run error: oops",
		Events: []oteltest.Event{
			{
				Name: "exception",
				Attrs: map[string]any{
					"exception.message": "task run error: oops",
					"exception.type":    "ErrTaskRun",
				},
			},
		},
	})

	// Second span is the eval span for the first failed case - should have Error status
	spans[1].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		StatusCode:        codes.Error,
		StatusDescription: "task run error: oops",
	})

	// Third span is the successful task for the second case
	spans[2].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.expected": "4",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      2,
			"braintrust.output_json":     2,
			"braintrust.span_attributes": taskType,
		},
	})

	// Fourth span is the score for the second case
	spans[3].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"Equals": 0},
			"braintrust.span_attributes": scoreType,
		},
	})

	// Fifth span is the eval for the second case
	spans[4].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.expected": "4",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      2,
			"braintrust.output_json":     2,
			"braintrust.span_attributes": evalType,
		},
	})
}

func TestEval_ScorerErrors(t *testing.T) {
	// a test that verifies we properly handle evals where some
	// scorers pass and some have errors.

	assert, require := assert.New(t), require.New(t)
	assert.NotNil(require)

	_, exporter := oteltest.Setup(t)

	cases := []Case[int, int]{
		{Input: 1, Expected: 1},
		{Input: 2, Expected: 4},
	}

	// Simple task that works correctly
	task := func(ctx context.Context, x int) (int, error) {
		return x * x, nil
	}

	// Mix of scorers - one that works and one that fails
	scorers := []Scorer[int, int]{
		autoevals.NewEquals[int, int](), // This works
		NewScorer("failing_scorer", func(ctx context.Context, input int, expected, result int) (float64, error) {
			if input == 2 {
				return 0, errors.New("scorer failed for input 2")
			}
			return 1.0, nil
		}),
	}

	eval := New("123", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run()
	timeRange := timer.Tick()

	assert.ErrorIs(err, ErrScorer)
	assert.Contains(err.Error(), "scorer failed for input 2")

	spans := exporter.Flush()
	// We get 6 spans: task1, score1, eval1, task2, score2, eval2
	assert.Equal(6, len(spans))

	for _, span := range spans {
		span.AssertInTimeRange(timeRange)
	}

	// First case (input=1, expected=1, result=1) - all succeed
	spans[0].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.expected": "1",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      1,
			"braintrust.output_json":     1,
			"braintrust.span_attributes": taskType,
		},
	})

	spans[1].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"Equals": 1, "failing_scorer": 1},
			"braintrust.span_attributes": scoreType,
		},
	})

	spans[2].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.expected": "1",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      1,
			"braintrust.output_json":     1,
			"braintrust.span_attributes": evalType,
		},
	})

	// Second case (input=2, expected=4, result=4) - scorer fails
	spans[3].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.expected": "4",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      2,
			"braintrust.output_json":     4,
			"braintrust.span_attributes": taskType,
		},
	})

	// Score span should have exception event for the failing scorer
	spans[4].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"Equals": 1},
			"braintrust.span_attributes": scoreType,
		},
		StatusCode:        codes.Error,
		StatusDescription: "scorer error: scorer \"failing_scorer\" failed: scorer failed for input 2",
		Events: []oteltest.Event{
			{
				Name: "exception",
				Attrs: map[string]any{
					"exception.message": "scorer error: scorer \"failing_scorer\" failed: scorer failed for input 2",
					"exception.type":    "ErrScorer",
				},
			},
		},
	})

	// Final eval span should have Error status due to scorer failure
	spans[5].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		StatusCode:        codes.Error,
		StatusDescription: "scorer error: scorer \"failing_scorer\" failed: scorer failed for input 2",
	})
}

func TestHardcodedEval(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, exporter := oteltest.Setup(t)

	// test task
	id := "123"

	brokenSquare := func(ctx context.Context, x int) (int, error) {
		square := x * x
		if x > 1 {
			square++ // oh no it's wrong
		}
		return square, nil
	}

	// test custom scorer
	equals := func(ctx context.Context, input int, expected, result int) (float64, error) {
		if result == expected {
			return 1.0, nil
		}
		return 0.0, nil
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 1, Tags: []string{"tag1", "tag2"}},
		{Input: 2, Expected: 4},
	}

	scorers := []Scorer[int, int]{
		NewScorer("equals", equals),
	}

	eval1 := New(id, NewCases(cases), brokenSquare, scorers)
	timer := oteltest.NewTimer()
	err := eval1.Run()
	timeRange := timer.Tick()
	require.NoError(err)

	spans := exporter.Flush()
	assert.Equal(len(cases)*3, len(spans))

	// Assert first case spans using new API
	spans[0].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.expected": "1",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      1,
			"braintrust.output_json":     1,
			"braintrust.span_attributes": taskType,
		},
		TimeRange: timeRange,
	})

	spans[1].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"equals": 1},
			"braintrust.span_attributes": scoreType,
		},
	})

	spans[2].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.expected": "1",
			"braintrust.parent":   "experiment_id:123",
			"braintrust.tags":     []string{"tag1", "tag2"},
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      1,
			"braintrust.output_json":     1,
			"braintrust.span_attributes": evalType,
		},
	})

	// Assert second case spans using new API
	spans[3].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.expected": "4",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      2,
			"braintrust.output_json":     5,
			"braintrust.span_attributes": taskType,
		},
	})

	spans[4].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"equals": 0},
			"braintrust.span_attributes": scoreType,
		},
	})

	spans[5].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.expected": "4",
			"braintrust.parent":   "experiment_id:123",
		},
		JSONAttrs: map[string]any{
			"braintrust.input_json":      2,
			"braintrust.output_json":     5,
			"braintrust.span_attributes": evalType,
		},
	})
}

func TestCases(t *testing.T) {
	assert := assert.New(t)

	// Test with empty slice
	emptyCases := NewCases([]Case[int, int]{})
	_, err := emptyCases.Next()
	assert.ErrorIs(err, io.EOF)

	// Test with populated slice
	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 3, Expected: 6},
	}

	sliceCases := NewCases(cases)

	// First case
	case1, err := sliceCases.Next()
	assert.NoError(err)
	assert.Equal(1, case1.Input)
	assert.Equal(2, case1.Expected)

	// Second case
	case2, err := sliceCases.Next()
	assert.NoError(err)
	assert.Equal(3, case2.Input)
	assert.Equal(6, case2.Expected)

	// No more cases - should return io.EOF
	_, err = sliceCases.Next()
	assert.ErrorIs(err, io.EOF)

	// Subsequent calls should also return io.EOF
	_, err = sliceCases.Next()
	assert.ErrorIs(err, io.EOF)
}

type intGenerator struct {
	start int
	end   int
}

func newIntGenerator(start, end int) *intGenerator {
	return &intGenerator{start: start, end: end}
}

func (g *intGenerator) Next() (Case[int, int], error) {
	if g.start >= g.end {
		return Case[int, int]{}, io.EOF
	}
	g.start++
	return Case[int, int]{Input: g.start, Expected: g.start}, nil
}

func TestEvalWithCustomGenerator(t *testing.T) {
	require := require.New(t)

	_, exporter := oteltest.Setup(t)

	generator := newIntGenerator(0, 2)

	task := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}

	scorers := []Scorer[int, int]{autoevals.NewEquals[int, int]()}

	eval := New("test-generator", generator, task, scorers)
	err := eval.Run()
	require.NoError(err)

	spans := exporter.Flush()
	require.Equal(6, len(spans)) // 2 cases * 3 spans each
}

func TestEvalWithCasesIteratorError(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, exporter := oteltest.Setup(t)

	// Create a generator that returns: case1, error, case2, EOF
	generator := &errCases{
		sequence: []errCase{
			{c: Case[string, string]{Input: "first", Expected: "first"}},
			{err: errors.New("iterator error between cases")},
			{c: Case[string, string]{Input: "second", Expected: "second"}},
		},
		index: 0,
	}

	task := func(ctx context.Context, input string) (string, error) {
		return input, nil
	}

	scorers := []Scorer[string, string]{autoevals.NewEquals[string, string]()}

	eval := New("test-error-generator", generator, task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run()
	timeRange := timer.Tick()

	// Should return the error from the Cases iterator
	require.Error(err)
	assert.Contains(err.Error(), "iterator error between cases")

	// Should have processed both successful cases plus one error span
	spans := exporter.Flush()
	assert.Equal(7, len(spans)) // 2 cases * 3 spans each + 1 iterator error span

	for _, span := range spans {
		span.AssertInTimeRange(timeRange)
	}

	// First case spans (first, first) - task span
	spans[0].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.expected":        "first",
			"braintrust.input_json":      "first",
			"braintrust.output_json":     "first",
			"braintrust.span_attributes": taskType,
		},
	})

	// First case spans (first, first) - score span
	spans[1].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"Equals": 1},
			"braintrust.span_attributes": scoreType,
		},
	})

	// First case spans (first, first) - eval span
	spans[2].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.expected":        "first",
			"braintrust.input_json":      "first",
			"braintrust.output_json":     "first",
			"braintrust.span_attributes": evalType,
		},
	})

	// Iterator error span - should be marked as error and have no input/output
	spans[3].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		StatusCode:        codes.Error,
		StatusDescription: "case iterator error: iterator error between cases",
		Events: []oteltest.Event{
			{
				Name: "exception",
				Attrs: map[string]any{
					"exception.message": "case iterator error: iterator error between cases",
					"exception.type":    "ErrCaseIterator",
				},
			},
		},
	})

	// Second case spans (second, second) - task span
	spans[4].AssertEqual(oteltest.TestSpan{
		Name: "task",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.expected":        "second",
			"braintrust.input_json":      "second",
			"braintrust.output_json":     "second",
			"braintrust.span_attributes": taskType,
		},
	})

	// Second case spans (second, second) - score span
	spans[5].AssertEqual(oteltest.TestSpan{
		Name: "score",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.scores":          map[string]int{"Equals": 1},
			"braintrust.span_attributes": scoreType,
		},
	})

	// Second case spans (second, second) - eval span
	spans[6].AssertEqual(oteltest.TestSpan{
		Name: "eval",
		Attrs: map[string]any{
			"braintrust.parent": "experiment_id:test-error-generator",
		},
		JSONAttrs: map[string]any{
			"braintrust.expected":        "second",
			"braintrust.input_json":      "second",
			"braintrust.output_json":     "second",
			"braintrust.span_attributes": evalType,
		},
	})
}

type errCase struct {
	c   Case[string, string]
	err error
}

type errCases struct {
	sequence []errCase
	index    int
}

func (e *errCases) Next() (Case[string, string], error) {
	if e.index >= len(e.sequence) {
		return Case[string, string]{}, io.EOF
	}

	step := e.sequence[e.index]
	e.index++

	return step.c, step.err
}

func TestResolveExperimentID_Validation(t *testing.T) {
	// Test empty experiment name
	_, err := ResolveExperimentID("", "test-project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment name is required")

	// Test empty project ID
	_, err = ResolveExperimentID("test-exp", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project ID is required")
}

func TestResolveProjectExperimentID_Validation(t *testing.T) {
	// Test empty experiment name
	_, err := ResolveProjectExperimentID("", "test-project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "experiment name is required")

	// Test empty project name
	_, err = ResolveProjectExperimentID("test-exp", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name is required")
}

func TestEval_EmptyExperimentID(t *testing.T) {
	// Test that an empty experiment ID produces an error span
	assert, require := assert.New(t), require.New(t)
	assert.NotNil(require)

	_, exporter := oteltest.Setup(t)

	// Simple task and scorer
	task := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}

	scorers := []Scorer[int, int]{
		autoevals.NewEquals[int, int](),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
	}

	// Create eval with empty experiment ID
	eval := New("", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run()
	timeRange := timer.Tick()

	// Should return ErrEval error
	assert.ErrorIs(err, ErrEval)
	assert.Contains(err.Error(), "experiment ID is required")

	spans := exporter.Flush()
	// Should have no spans since the eval fails immediately
	assert.Equal(0, len(spans))

	// Verify timer was used (even though no spans were created)
	_ = timeRange
}

func TestEval_BraintrustParentWithAndWithoutDefaultProject(t *testing.T) {

	tests := []struct {
		name          string
		withProcessor bool
		projectID     string
	}{
		{"with default project", true, "test-project-123"},
		{"without default project", true, ""},
		{"without processor", false, ""},
	}

	// ideally our customers always use our span processor which will handle
	// setting the bt parent on every span. but sometimes they won't. this test
	// validates that at least in experiments we set the parent on the eval spans,
	// since it's idempotent.

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []sdktrace.TracerProviderOption{}
			if tt.withProcessor {
				spanProcessorOpts := []trace.SpanProcessorOption{}
				if tt.projectID != "" {
					spanProcessorOpts = append(spanProcessorOpts, trace.WithDefaultProjectID(tt.projectID))
				}
				processor := trace.NewSpanProcessor(spanProcessorOpts...)
				opts = append(opts, sdktrace.WithSpanProcessor(processor))
			}
			_, exporter := oteltest.Setup(t, opts...)

			task := func(ctx context.Context, x int) (int, error) {
				return x * 2, nil
			}

			scorers := []Scorer[int, int]{
				autoevals.NewEquals[int, int](),
			}

			cases := []Case[int, int]{
				{Input: 1, Expected: 2},
			}

			eval := New("test-exp-456", NewCases(cases), task, scorers)
			err := eval.Run()
			require.NoError(t, err)

			spans := exporter.Flush()
			assert.Equal(t, 3, len(spans)) // task, score, eval

			// Verify all spans have braintrust.parent attribute
			for _, span := range spans {
				span.AssertAttrEquals("braintrust.parent", "experiment_id:test-exp-456")
			}
		})
	}
}
