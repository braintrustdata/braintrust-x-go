package eval

import (
	"context"
	"testing"

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
	"github.com/braintrust/braintrust-x-go/braintrust/internal"
	"github.com/braintrust/braintrust-x-go/braintrust/internal/testspan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setUpTest(t *testing.T) (*tracetest.InMemoryExporter, func()) {
	internal.FailTestsOnWarnings(t)

	// setup otel to be fully synchronous
	exporter := tracetest.NewInMemoryExporter()
	processor := trace.NewSimpleSpanProcessor(exporter)
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSpanProcessor(processor), // flushes immediately
	)

	original := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	teardown := func() {
		diag.ClearLogger()
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error shutting down tracer provider: %v", err)
		}
		otel.SetTracerProvider(original)
	}

	return exporter, teardown
}

func TestHardcodedEval(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	assert.True(true)
	require.True(true)

	exporter, teardown := setUpTest(t)
	defer teardown()

	// here's a fake task
	task := func(ctx context.Context, x int) (int, error) {
		square := x * x
		if x > 3 {
			square += 1 // oh no it's wrong
		}
		return square, nil
	}

	// here's a fake scorer
	equals := func(ctx context.Context, c Case[int, int], actual int) (float64, error) {
		if actual == c.Expected {
			return 1.0, nil
		}
		return 0.0, nil
	}

	cases := []Case[int, int]{
		{Input: 1, Expected: 1},
		{Input: 2, Expected: 4},
		{Input: 3, Expected: 9},
		{Input: 4, Expected: 16},
	}

	scorers := []Scorer[int, int]{
		NewScorer("equals", equals),
	}

	eval := New("test", cases, task, scorers)

	err := eval.Run()
	require.NoError(err)

	spans := testspan.Flush(t, exporter)
	assert.NotEmpty(spans)

	// for _, span := range spans {
	// 	//fmt.Println(span)
	// }
}
