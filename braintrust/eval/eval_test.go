package eval

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
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

	eval := New("experiment_id:123", NewCases(cases), task, scorers)
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
	assert.JSONEq(`{
		"attributes": {},
		"events": [
			{
				"attributes": {
					"exception.message": "task run error: oops",
					"exception.type": "*fmt.wrapErrors"
				},
				"name": "exception"
			}
		],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Error",
			"description": "task run error: oops"
		}
	}`, spans[0].Snapshot())

	// Second span is the eval span for the first failed case - should have Error status
	assert.JSONEq(`{
		"attributes": {},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Error",
			"description": "task run error: oops"
		}
	}`, spans[1].Snapshot())

	// Third span is the successful task for the second case
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "4",
			"braintrust.input_json": "2",
			"braintrust.output_json": "2",
			"braintrust.span_attributes": "{\"type\":\"task\"}"
		},
		"events": [],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[2].Snapshot())

	// Fourth span is the score for the second case
	assert.JSONEq(`{
		"attributes": {
			"braintrust.scores": "{\"Equals\":0}",
			"braintrust.span_attributes": "{\"type\":\"score\"}"
		},
		"events": [],
		"name": "score",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[3].Snapshot())

	// Fifth span is the eval for the second case
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "4",
			"braintrust.input_json": "2",
			"braintrust.output_json": "2",
			"braintrust.span_attributes": "{\"type\":\"eval\"}"
		},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[4].Snapshot())
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

	eval := New("experiment_id:123", NewCases(cases), task, scorers)
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
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "1",
			"braintrust.input_json": "1",
			"braintrust.output_json": "1",
			"braintrust.span_attributes": "{\"type\":\"task\"}"
		},
		"events": [],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[0].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.scores": "{\"Equals\":1,\"failing_scorer\":1}",
			"braintrust.span_attributes": "{\"type\":\"score\"}"
		},
		"events": [],
		"name": "score",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[1].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "1",
			"braintrust.input_json": "1",
			"braintrust.output_json": "1",
			"braintrust.span_attributes": "{\"type\":\"eval\"}"
		},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[2].Snapshot())

	// Second case (input=2, expected=4, result=4) - scorer fails
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "4",
			"braintrust.input_json": "2",
			"braintrust.output_json": "4",
			"braintrust.span_attributes": "{\"type\":\"task\"}"
		},
		"events": [],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[3].Snapshot())

	// Score span should have exception event for the failing scorer
	assert.JSONEq(`{
		"attributes": {
			"braintrust.scores": "{\"Equals\":1}",
			"braintrust.span_attributes": "{\"type\":\"score\"}"
		},
		"events": [
			{
				"attributes": {
					"exception.message": "scorer error: scorer \"failing_scorer\" failed: scorer failed for input 2",
					"exception.type": "*fmt.wrapErrors"
				},
				"name": "exception"
			}
		],
		"name": "score",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[4].Snapshot())

	// Final eval span should have Error status due to scorer failure
	assert.JSONEq(`{
		"attributes": {},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Error",
			"description": "scorer error: scorer error: scorer \"failing_scorer\" failed: scorer failed for input 2"
		}
	}`, spans[5].Snapshot())
}

func TestHardcodedEval(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	_, exporter := oteltest.Setup(t)

	// test task
	id := "experiment_id:123"

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
		{Input: 1, Expected: 1},
		{Input: 2, Expected: 4},
	}

	scorers := []Scorer[int, int]{
		NewScorer("equals", equals),
	}

	eval1 := New(id, NewCases(cases), brokenSquare, scorers)
	err := eval1.Run()
	require.NoError(err)

	spans := exporter.Flush()
	assert.Equal(len(cases)*3, len(spans))

	// Assert first case spans
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "1",
			"braintrust.input_json": "1",
			"braintrust.output_json": "1",
			"braintrust.span_attributes": "{\"type\":\"task\"}"
		},
		"events": [],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[0].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.scores": "{\"equals\":1}",
			"braintrust.span_attributes": "{\"type\":\"score\"}"
		},
		"events": [],
		"name": "score",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[1].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "1",
			"braintrust.input_json": "1",
			"braintrust.output_json": "1",
			"braintrust.span_attributes": "{\"type\":\"eval\"}"
		},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[2].Snapshot())

	// Assert second case spans
	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "4",
			"braintrust.input_json": "2",
			"braintrust.output_json": "5",
			"braintrust.span_attributes": "{\"type\":\"task\"}"
		},
		"events": [],
		"name": "task",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[3].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.scores": "{\"equals\":0}",
			"braintrust.span_attributes": "{\"type\":\"score\"}"
		},
		"events": [],
		"name": "score",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[4].Snapshot())

	assert.JSONEq(`{
		"attributes": {
			"braintrust.expected": "4",
			"braintrust.input_json": "2",
			"braintrust.output_json": "5",
			"braintrust.span_attributes": "{\"type\":\"eval\"}"
		},
		"events": [],
		"name": "eval",
		"spanKind": "internal",
		"status": {
			"code": "Unset",
			"description": ""
		}
	}`, spans[5].Snapshot())
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
	err := eval.Run()

	// Should return the error from the Cases iterator
	require.Error(err)
	assert.Contains(err.Error(), "iterator error between cases")

	// Should have processed both successful cases plus one error span
	spans := exporter.Flush()
	assert.Equal(7, len(spans)) // 2 cases * 3 spans each + 1 iterator error span

	// Verify the spans are for the correct cases
	// First case spans (first, first)
	assert.Equal("task", spans[0].Name())
	assert.Equal("first", spans[0].Input())
	assert.Equal("first", spans[0].Output())

	assert.Equal("score", spans[1].Name())
	assert.Contains(spans[1].Attr("braintrust.span_attributes").String(), "score")

	assert.Equal("eval", spans[2].Name())
	assert.Equal("first", spans[2].Input())
	assert.Equal("first", spans[2].Output())

	// Iterator error span - should be marked as error and have no input/output
	assert.Equal("eval", spans[3].Name())
	assert.Equal("Error", spans[3].Status().Code.String())
	assert.Contains(spans[3].Status().Description, "case iterator error")
	// Error span should not have input/output attributes since the iterator failed
	assert.False(spans[3].HasAttr("braintrust.input_json"))
	assert.False(spans[3].HasAttr("braintrust.output_json"))

	// Second case spans (second, second)
	assert.Equal("task", spans[4].Name())
	assert.Equal("second", spans[4].Input())
	assert.Equal("second", spans[4].Output())

	assert.Equal("score", spans[5].Name())
	assert.Contains(spans[5].Attr("braintrust.span_attributes").String(), "score")

	assert.Equal("eval", spans[6].Name())
	assert.Equal("second", spans[6].Input())
	assert.Equal("second", spans[6].Output())
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
