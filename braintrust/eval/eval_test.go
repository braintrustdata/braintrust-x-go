package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/braintrust/braintrust-x-go/braintrust/autoevals"
	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
)

func TestEvalTaskError(t *testing.T) {
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
		return 0, errors.New("error running task")
	}

	scorers := []Scorer[int, int]{
		autoevals.NewEquals[int, int](),
	}

	eval := New("experiment_id:123", cases, task, scorers)
	err := eval.Run()
	require.Error(err)

	spans := exporter.Flush()
	assert.Equal(len(cases)*3, len(spans))
	for _, span := range spans {
		assert.Equal("task", span.Name())
	}
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
			square += 1 // oh no it's wrong
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

	eval1 := New(id, cases, brokenSquare, scorers)
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
