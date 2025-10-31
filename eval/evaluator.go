package eval

import (
	"context"

	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
)

// Evaluator provides a reusable way to run multiple evaluations with the same
// input and output types. This is useful when you need to run several evaluations
// in sequence with the same type signature.
//
// Evaluator is created with explicit dependencies (session, config, tracerProvider)
// for maximum flexibility. Most users should use braintrust.NewEvaluator(client)
// which extracts these dependencies from the client.
//
// Example (using primitive constructor):
//
//	evaluator := eval.NewEvaluator[string, string](session, config, tp)
//	result, _ := evaluator.Run(ctx, eval.Opts[string, string]{
//	    Experiment: "test-1",
//	    Cases:      cases1,
//	    Task:       task1,
//	    Scorers:    scorers,
//	})
type Evaluator[I, R any] struct {
	session        *auth.Session
	config         *config.Config
	tracerProvider *trace.TracerProvider
}

// NewEvaluator creates a new evaluator with explicit dependencies.
// The type parameters I (input) and R (result/output) must be specified explicitly.
//
// Most users should use braintrust.NewEvaluator(client) which extracts these
// dependencies from the client for convenience.
//
// Example:
//
//	evaluator := eval.NewEvaluator[string, string](session, config, tp)
func NewEvaluator[I, R any](session *auth.Session, cfg *config.Config, tp *trace.TracerProvider) *Evaluator[I, R] {
	return &Evaluator[I, R]{
		session:        session,
		config:         cfg,
		tracerProvider: tp,
	}
}

// Run executes an evaluation using this evaluator's dependencies.
func (e *Evaluator[I, R]) Run(ctx context.Context, opts Opts[I, R]) (*Result, error) {
	return Run(ctx, opts, e.config, e.session, e.tracerProvider)
}
