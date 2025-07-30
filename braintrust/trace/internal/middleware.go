// Package internal provides shared middleware functionality for OpenTelemetry tracers.
package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/braintrustdata/braintrust-x-go/braintrust/log"
)

// MiddlewareTracer adds tracing to API requests by parsing bodies of the request and response.
type MiddlewareTracer interface {
	StartSpan(ctx context.Context, start time.Time, request io.Reader) (context.Context, trace.Span, error)
	TagSpan(span trace.Span, response io.Reader) error
}

// NextMiddleware represents the next middleware to run in the client middleware chain.
type NextMiddleware = func(req *http.Request) (*http.Response, error)

// TracerRouter maps URL paths to specific tracers for different endpoints.
type TracerRouter func(path string) MiddlewareTracer

// Middleware creates a shared OpenTelemetry middleware that uses the provided router
// to determine which tracer to use for each endpoint.
func Middleware(router TracerRouter) func(*http.Request, NextMiddleware) (*http.Response, error) {
	return func(req *http.Request, next NextMiddleware) (*http.Response, error) {
		log.Debugf("Middleware: %s %s", req.Method, req.URL.Path)
		start := time.Now()

		// Intercept the request body so we can parse it and still pass it along.
		var buf bytes.Buffer
		reqBody := req.Body
		defer func() {
			if err := reqBody.Close(); err != nil {
				log.Warnf("Error closing request body: %v", err)
			}
		}()
		tee := io.TeeReader(reqBody, &buf)
		req.Body = io.NopCloser(&buf)

		// Start a span with data parsed from the request.
		var reqTracer MiddlewareTracer = NewNoopTracer()
		if req.URL != nil {
			reqTracer = router(req.URL.Path)
		}

		ctx, span, err := reqTracer.StartSpan(req.Context(), start, tee)
		req = req.WithContext(ctx)
		if err != nil {
			log.Warnf("Error starting span: %v", err)
		}

		// Continue processing the request.
		resp, err := next(req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			return resp, err
		}

		// Intercept the response body, so we can gather tracing data.
		//
		// It's critical that we don't try to parse the whole response body here because
		// we don't want to block clients waiting for streaming responses.
		onResponseDone := func(r io.Reader) {
			// NOTE: this could be done in a goroutine so we don't add any extra
			// latency to the response.
			now := time.Now()
			if err := reqTracer.TagSpan(span, r); err != nil {
				log.Warnf("Error tagging span: %v\n%s", err)
			}
			span.End(trace.WithTimestamp(now))
		}
		body := NewBufferedReader(resp.Body, onResponseDone)
		resp.Body = body
		return resp, nil
	}
}

// NoopTracer is a MiddlewareTracer that doesn't record any tracing data.
type NoopTracer struct{}

// NewNoopTracer creates a new noop tracer.
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// StartSpan creates a non-recording span for the NoopTracer.
func (*NoopTracer) StartSpan(ctx context.Context, _ time.Time, _ io.Reader) (context.Context, trace.Span, error) {
	span := trace.SpanFromContext(context.Background()) // create a non-recording span
	return ctx, span, nil
}

// TagSpan does nothing for the NoopTracer.
func (*NoopTracer) TagSpan(_ trace.Span, _ io.Reader) error {
	return nil
}

var _ MiddlewareTracer = &NoopTracer{}
