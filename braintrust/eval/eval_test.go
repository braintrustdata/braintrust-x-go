package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/braintrust/braintrust-x-go/braintrust/internal/oteltest"
)

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

	// test scorer
	equals := func(ctx context.Context, c Case[int, int], actual int) (float64, error) {
		if actual == c.Expected {
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
