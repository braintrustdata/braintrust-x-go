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
	var reqTracer httpTracer
	if req.URL != nil {
		switch req.URL.Path {
		case "/v1/responses":
			reqTracer = newResponsesTracer()
		default:
			reqTracer = newNoopHTTPTracer()
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

	// Intercept the response body so we can parse and return it.
	respBody := resp.Body
	r1, r2 := Tee(respBody)
	resp.Body = r1

	go func() {
		err = reqTracer.TagSpan(span, r2)
		if err != nil {
			logger.Warnf("Error tagging span: %v", err)
		}
		span.End()
	}()

	return resp, nil
}

type httpTracer interface {
	StartSpan(ctx context.Context, start time.Time, request io.Reader) (context.Context, trace.Span, error)
	TagSpan(span trace.Span, response io.Reader) error
}

// noopHTTPTracer is an httpTracer that doesn't record any tracing data.
type noopHTTPTracer struct{}

func newNoopHTTPTracer() *noopHTTPTracer {
	return &noopHTTPTracer{}
}

func (*noopHTTPTracer) StartSpan(ctx context.Context, start time.Time, request io.Reader) (context.Context, trace.Span, error) {
	span := trace.SpanFromContext(context.Background()) // create a non-recording span
	return ctx, span, nil
}

func (*noopHTTPTracer) TagSpan(span trace.Span, response io.Reader) error {
	return nil
}

var _ httpTracer = &noopHTTPTracer{}
