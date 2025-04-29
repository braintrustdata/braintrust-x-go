package traceopenai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

type NextMiddleware = func(req *http.Request) (*http.Response, error)

func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {
	start := time.Now()

	logger := logger()
	logger.Debugf("Middleware: %s %s", req.Method, req.URL.Path)

	// Intercept the request body so we can parse it and still pass it along.
	var buf bytes.Buffer
	reqBody := req.Body
	defer reqBody.Close()
	tee := io.TeeReader(reqBody, &buf)
	req.Body = io.NopCloser(&buf)

	// Start a span with data parsed from the request.
	var reqTracer middlewareTracer = newNoopTracer()
	if req.URL != nil {
		switch req.URL.Path {
		case "/v1/responses":
			reqTracer = newResponsesTracer()
		}
	}

	ctx, span, err := reqTracer.StartSpan(req.Context(), start, tee)
	req = req.WithContext(ctx)
	if err != nil {
		logger.Warnf("Error starting span: %v", err)
	}

	// Continue processing the request.
	resp, err := next(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return resp, err
	}

	onResponseDone := func(r io.Reader) {
		now := time.Now()
		if err := reqTracer.TagSpan(span, r); err != nil {
			logger.Warnf("Error tagging span: %v\n%s", err)
		}
		span.End(trace.WithTimestamp(now))
	}
	body := newBufferedReader(resp.Body, onResponseDone)
	resp.Body = body
	return resp, nil
}

// middlewareTracer adds tracing to openai requests by parsing bodies of the request and response.
type middlewareTracer interface {
	StartSpan(ctx context.Context, start time.Time, request io.Reader) (context.Context, trace.Span, error)
	TagSpan(span trace.Span, response io.Reader) error
}

// noopTracer is a middlewareTracer that doesn't record any tracing data.
type noopTracer struct{}

func newNoopTracer() *noopTracer {
	return &noopTracer{}
}

func (*noopTracer) StartSpan(ctx context.Context, start time.Time, request io.Reader) (context.Context, trace.Span, error) {
	span := trace.SpanFromContext(context.Background()) // create a non-recording span
	return ctx, span, nil
}

func (*noopTracer) TagSpan(span trace.Span, response io.Reader) error {
	return nil
}

var _ middlewareTracer = &noopTracer{}
