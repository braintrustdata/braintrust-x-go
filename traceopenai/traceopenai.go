package traceopenai

import (
	"bytes"
	"context"
	"io"
	"log"
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

	// Intercept the request body so we can parse it and still pass it along.
	var err error
	var body []byte
	body, req.Body, err = cloneBody(req.Body)
	if err != nil {
		return nil, err
	}

	// Start a span with data parsed from the request.
	var reqTracer httpTracer
	if req.URL != nil {
		switch req.URL.Path {
		case "/v1/responses":
			reqTracer = NewV1ResponsesTracer()
		default:
			reqTracer = NewNoopHTTPTracer()
		}
	}

	ctx, span, err := reqTracer.startSpanFromRequest(req.Context(), start, body)
	defer func() {
		span.End()
	}()
	req = req.WithContext(ctx)
	if err != nil {
		// Proceed if there's an error in tracing code
		log.Printf("Error starting span: %v", err)
	}

	// Continue processing the request.
	resp, err := next(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	// Intercept the response body so we can parse and return it.
	body, resp.Body, err = cloneBody(resp.Body)
	if err != nil {
		return resp, err
	}

	err = reqTracer.tagSpanWithResponse(span, body)
	if err != nil {
		// Don't fail for tracing errors.
		log.Printf("Error tagging span: %v", err)
	}

	return resp, nil
}

func cloneBody(r io.ReadCloser) ([]byte, io.ReadCloser, error) {
	if r == nil {
		return nil, nil, nil
	}
	bodyBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	return bodyBytes, io.NopCloser(bytes.NewReader(bodyBytes)), nil
}

type httpTracer interface {
	startSpanFromRequest(ctx context.Context, start time.Time, body []byte) (context.Context, trace.Span, error)
	tagSpanWithResponse(span trace.Span, body []byte) error
}

// noopHTTPTracer is an httpTracer that doesn't record any tracing data.
type noopHTTPTracer struct{}

func NewNoopHTTPTracer() *noopHTTPTracer {
	return &noopHTTPTracer{}
}

func (*noopHTTPTracer) startSpanFromRequest(ctx context.Context, start time.Time, body []byte) (context.Context, trace.Span, error) {
	span := trace.SpanFromContext(context.Background()) // create a non-recording span
	return ctx, span, nil
}

func (*noopHTTPTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	return nil
}

var _ httpTracer = &noopHTTPTracer{}
