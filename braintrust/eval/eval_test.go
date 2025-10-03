package eval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/api"
	"github.com/braintrustdata/braintrust-x-go/braintrust/internal/oteltest"
)

var (
	scoreType = map[string]string{"type": "score"}
	evalType  = map[string]string{"type": "eval"}
	taskType  = map[string]string{"type": "task"}
)

func entropy() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

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
		NewEqualsScorer[int, int](),
	}

	eval := New("123", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run(context.Background())
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
			"braintrust.scores":          map[string]int{"equals": 0},
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
		NewEqualsScorer[int, int](),
		NewScorer("failing_scorer", func(ctx context.Context, input int, expected, result int, _ Metadata) (Scores, error) {
			if input == 2 {
				return nil, errors.New("scorer failed for input 2")
			}
			return Scores{{Name: "failing_scorer", Score: 1.0}}, nil
		}),
	}

	eval := New("123", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run(context.Background())
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
			"braintrust.scores":          map[string]float64{"equals": 1, "failing_scorer": 1},
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
			"braintrust.scores":          map[string]int{"equals": 1},
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

func TestScorerNames(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	_, exporter := oteltest.Setup(t)

	scorers := []Scorer[int, int]{
		NewScorer("no-name", func(ctx context.Context, input, expected, result int, _ Metadata) (Scores, error) {
			return S(0.6), nil
		}),
		NewScorer("name", func(ctx context.Context, input, expected, result int, _ Metadata) (Scores, error) {
			return Scores{{Name: "different", Score: 0.5}}, nil
		}),
	}

	task := func(ctx context.Context, input int) (int, error) {
		return input, nil
	}

	cases := []Case[int, int]{{Input: 1, Expected: 1}}

	eval := New("123", NewCases(cases), task, scorers)
	err := eval.Run(context.Background())
	require.NoError(err)

	spans := exporter.Flush()
	assert.Equal(3, len(spans))

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
			"braintrust.scores":          map[string]float64{"different": 0.5, "no-name": 0.6},
			"braintrust.span_attributes": scoreType,
		},
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
	equals := func(ctx context.Context, input, expected, result int, _ Metadata) (Scores, error) {
		v := 0.0
		if result == expected {
			v = 1.0
		}
		return S(v), nil
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
	err := eval1.Run(context.Background())
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

	scorers := []Scorer[int, int]{NewEqualsScorer[int, int]()}

	eval := New("test-generator", generator, task, scorers)
	err := eval.Run(context.Background())
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

	scorers := []Scorer[string, string]{NewEqualsScorer[string, string]()}

	eval := New("test-error-generator", generator, task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run(context.Background())
	timeRange := timer.Tick()

	// Should return the error from the Cases iterator
	require.Error(err)
	assert.NotNil(err)
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
			"braintrust.scores":          map[string]int{"equals": 1},
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
			"braintrust.scores":          map[string]int{"equals": 1},
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
		NewEqualsScorer[int, int](),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
	}

	// Create eval with empty experiment ID
	eval := New("", NewCases(cases), task, scorers)
	timer := oteltest.NewTimer()
	err := eval.Run(context.Background())
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

func TestEval_WithParallelism(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	_, exporter := oteltest.Setup(t)

	var timerLock sync.Mutex
	var timeRanges []oteltest.TimeRange

	task := func(ctx context.Context, x int) (int, error) {
		timer := oteltest.NewTimer()
		time.Sleep(100 * time.Millisecond)
		tr := timer.Tick()
		timerLock.Lock()
		timeRanges = append(timeRanges, tr)
		timerLock.Unlock()
		return x, nil
	}

	scorers := []Scorer[int, int]{
		NewEqualsScorer[int, int](),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 2, Expected: 4},
	}

	eval := New("test-exp-456", NewCases(cases), task, scorers)
	eval.setParallelism(2)
	err := eval.Run(context.Background())
	require.NoError(err)

	spans := exporter.Flush()
	assert.Equal(6, len(spans))

	var tasks []oteltest.Span
	for _, span := range spans {
		if span.Name() == "task" {
			tasks = append(tasks, span)
		}
	}
	assert.Equal(len(cases), len(tasks))
	assert.Equal(len(cases), len(timeRanges))
	assert.Equal(2, len(timeRanges))

	// assert both tasks start before either are finished.
	tr1, tr2 := timeRanges[0], timeRanges[1]
	assert.Less(tr1.Start, tr1.End)
	assert.Less(tr1.Start, tr2.End)
	assert.Less(tr2.Start, tr1.End)
	assert.Less(tr2.Start, tr2.End)
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
			var opts []braintrust.Option
			if tt.projectID != "" {
				opts = append(opts, braintrust.WithDefaultProjectID(tt.projectID))
			}
			_, exporter := oteltest.Setup(t, opts...)

			task := func(ctx context.Context, x int) (int, error) {
				return x * 2, nil
			}

			scorers := []Scorer[int, int]{
				NewEqualsScorer[int, int](),
			}

			cases := []Case[int, int]{
				{Input: 1, Expected: 2},
			}

			eval := New("test-exp-456", NewCases(cases), task, scorers)
			err := eval.Run(context.Background())
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

// we don't use the autoeval scorer here to avoid a circular dependency in tests
type equalsScorer[I, R comparable] struct{}

func (s *equalsScorer[I, R]) Name() string {
	return "equals"
}

func (s *equalsScorer[I, R]) Run(ctx context.Context, input I, expected, result R, _ Metadata) (Scores, error) {
	v := 0.0
	if result == expected {
		v = 1.0
	}
	return S(v), nil
}

func NewEqualsScorer[I, R comparable]() Scorer[I, R] {
	return &equalsScorer[I, R]{}
}

func TestRun_Validation(t *testing.T) {
	ctx := context.Background()

	task := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}
	scorers := []Scorer[int, int]{NewEqualsScorer[int, int]()}
	cases := NewCases([]Case[int, int]{{Input: 1, Expected: 2}})

	tests := []struct {
		name        string
		opts        Opts[int, int]
		expectedErr string
	}{
		{
			name:        "missing task",
			opts:        Opts[int, int]{Project: "p", Experiment: "e", Cases: cases, Scorers: scorers},
			expectedErr: "Task is required",
		},
		{
			name:        "missing scorers",
			opts:        Opts[int, int]{Project: "p", Experiment: "e", Cases: cases, Task: task},
			expectedErr: "at least one Scorer is required",
		},
		{
			name:        "missing experiment",
			opts:        Opts[int, int]{Project: "p", Cases: cases, Task: task, Scorers: scorers},
			expectedErr: "Experiment is required",
		},
		{
			name:        "missing cases",
			opts:        Opts[int, int]{Project: "p", Experiment: "e", Task: task, Scorers: scorers},
			expectedErr: "one of Cases, Dataset, or DatasetID is required",
		},
		{
			name:        "multiple case sources",
			opts:        Opts[int, int]{Project: "p", Experiment: "e", Cases: cases, Dataset: "dataset1", Task: task, Scorers: scorers},
			expectedErr: "only one of Cases, Dataset, or DatasetID should be provided",
		},
		{
			name:        "multiple case sources with DatasetID",
			opts:        Opts[int, int]{Project: "p", Experiment: "e", Cases: cases, DatasetID: "ds-123", Task: task, Scorers: scorers},
			expectedErr: "only one of Cases, Dataset, or DatasetID should be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Run(ctx, tt.opts)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestEval_ParallelWithTaskErrors(t *testing.T) {
	// Test that parallel execution properly handles multiple task errors
	require := require.New(t)
	assert := assert.New(t)
	_, exporter := oteltest.Setup(t)

	// Task that fails for even inputs
	task := func(ctx context.Context, x int) (int, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		if x%2 == 0 {
			return 0, errors.New("task failed for even input")
		}
		return x * 2, nil
	}

	scorers := []Scorer[int, int]{
		NewEqualsScorer[int, int](),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 2, Expected: 4},
		{Input: 3, Expected: 6},
		{Input: 4, Expected: 8},
	}

	eval := New("test-parallel-errors", NewCases(cases), task, scorers)
	eval.setParallelism(3)
	err := eval.Run(context.Background())

	// Should return errors from failed tasks
	require.Error(err)
	assert.Contains(err.Error(), "task failed for even input")

	spans := exporter.Flush()
	// 4 cases: 2 successful (3 spans each) + 2 failed (2 spans each: task + eval)
	// Total: 2*3 + 2*2 = 10 spans
	assert.Equal(10, len(spans))
}

func TestEval_ParallelWithScorerErrors(t *testing.T) {
	// Test that parallel execution properly handles multiple scorer errors
	require := require.New(t)
	assert := assert.New(t)
	_, exporter := oteltest.Setup(t)

	// Task that always succeeds
	task := func(ctx context.Context, x int) (int, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return x * 2, nil
	}

	// Scorer that fails for even results
	scorers := []Scorer[int, int]{
		NewScorer("conditional_scorer", func(ctx context.Context, input, expected, result int, _ Metadata) (Scores, error) {
			if result%2 == 0 && result > 2 {
				return nil, errors.New("scorer failed for result")
			}
			return S(1.0), nil
		}),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 2, Expected: 4},
		{Input: 3, Expected: 6},
		{Input: 4, Expected: 8},
	}

	eval := New("test-parallel-scorer-errors", NewCases(cases), task, scorers)
	eval.setParallelism(3)
	err := eval.Run(context.Background())

	// Should return errors from failed scorers
	require.Error(err)
	assert.Contains(err.Error(), "scorer failed for result")

	spans := exporter.Flush()
	// All tasks succeed, but some scorers fail
	// 4 cases * 3 spans each = 12 spans
	assert.Equal(12, len(spans))
}

func TestEval_ParallelAllTasksFail(t *testing.T) {
	// Test that parallel execution handles the case where all tasks fail
	require := require.New(t)
	assert := assert.New(t)
	_, exporter := oteltest.Setup(t)

	// Task that always fails
	task := func(ctx context.Context, x int) (int, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return 0, errors.New("task always fails")
	}

	scorers := []Scorer[int, int]{
		NewEqualsScorer[int, int](),
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 2},
		{Input: 2, Expected: 4},
		{Input: 3, Expected: 6},
	}

	eval := New("test-parallel-all-fail", NewCases(cases), task, scorers)
	eval.setParallelism(2)
	err := eval.Run(context.Background())

	// Should return all errors
	require.Error(err)
	assert.Contains(err.Error(), "task always fails")

	spans := exporter.Flush()
	// 3 cases * 2 spans each (task + eval, no scores) = 6 spans
	// All spans should be errors since all tasks fail
	assert.Equal(6, len(spans))
}

func TestEval_ParallelWithIteratorErrors(t *testing.T) {
	// Test that parallel execution properly handles iterator errors mixed with successful cases
	require := require.New(t)
	assert := assert.New(t)
	_, exporter := oteltest.Setup(t)

	// Create a generator that returns: case1, case2, error, case3, EOF
	generator := &errCases{
		sequence: []errCase{
			{c: Case[string, string]{Input: "first", Expected: "first"}},
			{c: Case[string, string]{Input: "second", Expected: "second"}},
			{err: errors.New("iterator error during parallel execution")},
			{c: Case[string, string]{Input: "third", Expected: "third"}},
		},
		index: 0,
	}

	task := func(ctx context.Context, input string) (string, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return input, nil
	}

	scorers := []Scorer[string, string]{NewEqualsScorer[string, string]()}

	eval := New("test-parallel-iterator-errors", generator, task, scorers)
	eval.setParallelism(2)
	err := eval.Run(context.Background())

	// Should return the iterator error
	require.Error(err)
	assert.Contains(err.Error(), "iterator error during parallel execution")

	spans := exporter.Flush()
	// 3 successful cases * 3 spans + 1 iterator error span = 10 spans
	assert.Equal(10, len(spans))
}

func TestRun_WithDatasetID(t *testing.T) {
	// Integration test that uses DatasetID option in eval.Run
	require := require.New(t)
	assert := assert.New(t)

	_, exporter := oteltest.Setup(t)

	// Create a project
	projectName := entropy()
	project, err := api.RegisterProject(projectName)
	require.NoError(err)

	// Create a dataset
	datasetInfo, err := api.CreateDataset(api.DatasetRequest{
		ProjectID:   project.ID,
		Name:        "test-dataset-id-" + projectName,
		Description: "Test dataset for eval.Run with DatasetID",
	})
	require.NoError(err)
	defer func() {
		_ = api.DeleteDataset(datasetInfo.ID)
	}()

	// Insert test data
	events := []api.DatasetEvent{
		{
			Input:    2,
			Expected: 4,
		},
		{
			Input:    5,
			Expected: 10,
		},
	}
	err = api.InsertDatasetEvents(datasetInfo.ID, events)
	require.NoError(err)

	// Run eval using DatasetID - this tests the DatasetID resolution path
	_, err = Run(context.Background(), Opts[int, int]{
		ProjectID:  project.ID,
		Experiment: "test-dataset-id-experiment",
		DatasetID:  datasetInfo.ID, // Using DatasetID directly
		Task: func(ctx context.Context, input int) (int, error) {
			return input * 2, nil
		},
		Scorers: []Scorer[int, int]{
			NewEqualsScorer[int, int](),
		},
	})
	require.NoError(err)

	// Verify spans were created
	spans := exporter.Flush()
	assert.Equal(6, len(spans)) // 2 cases * 3 spans each
}

func TestRun_WithDatasetName(t *testing.T) {
	// Integration test that uses Dataset name option in eval.Run
	require := require.New(t)
	assert := assert.New(t)

	_, exporter := oteltest.Setup(t)

	// Create a unique project name
	projectName := entropy()
	project, err := api.RegisterProject(projectName)
	require.NoError(err)

	// Create a dataset with unique name
	datasetName := "test-dataset-name-" + projectName
	datasetInfo, err := api.CreateDataset(api.DatasetRequest{
		ProjectID:   project.ID,
		Name:        datasetName,
		Description: "Test dataset for eval.Run with Dataset name",
	})
	require.NoError(err)
	defer func() {
		_ = api.DeleteDataset(datasetInfo.ID)
	}()

	// Insert test data
	events := []api.DatasetEvent{
		{
			Input:    3,
			Expected: 9,
		},
		{
			Input:    4,
			Expected: 16,
		},
	}
	err = api.InsertDatasetEvents(datasetInfo.ID, events)
	require.NoError(err)

	// Run eval using Dataset name - this tests the Dataset name resolution path
	_, err = Run(context.Background(), Opts[int, int]{
		Project:    projectName,
		Experiment: "test-dataset-name-experiment",
		Dataset:    datasetName, // Using Dataset name with automatic resolution
		Task: func(ctx context.Context, input int) (int, error) {
			return input * input, nil
		},
		Scorers: []Scorer[int, int]{
			NewEqualsScorer[int, int](),
		},
	})
	require.NoError(err)

	// Verify spans were created
	spans := exporter.Flush()
	assert.Equal(6, len(spans)) // 2 cases * 3 spans each
}
