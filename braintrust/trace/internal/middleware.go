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
func Middleware(getMiddlewareTracer TracerRouter) func(*http.Request, NextMiddleware) (*http.Response, error) {
	return func(req *http.Request, next NextMiddleware) (*http.Response, error) {
		log.Debugf("Middleware: %s %s", req.Method, req.URL.Path)
		start := time.Now()

		// Determine which tracer to use first
		var mt MiddlewareTracer
		if req.URL != nil {
			mt = getMiddlewareTracer(req.URL.Path)
		}

		var ctx context.Context
		var span trace.Span
		var err error

		if mt == nil {
			// Some endpoints aren't traced. Just pass them along.
			return next(req)
		}

		// Supported endpoint, let's set up tracing
		var buf bytes.Buffer
		reqBody := req.Body
		defer func() {
			if err := reqBody.Close(); err != nil {
				log.Warnf("Error closing request body: %v", err)
			}
		}()

		// Use TeeReader - as the tracer reads from tee, it will populate buf
		tee := io.TeeReader(reqBody, &buf)

		ctx, span, err = mt.StartSpan(req.Context(), start, tee)
		if err != nil {
			log.Warnf("Error starting span: %v", err)
		}

		// After tracer has read from tee, set request body to read from buffer
		req.Body = io.NopCloser(&buf)
		req = req.WithContext(ctx)

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
			if err := mt.TagSpan(span, r); err != nil {
				log.Warnf("Error tagging span: %v\n%s", err)
			}
			span.End(trace.WithTimestamp(now))
		}
		body := NewBufferedReader(resp.Body, onResponseDone)
		resp.Body = body
		return resp, nil
	}
}
