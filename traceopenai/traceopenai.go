package traceopenai

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

type NextMiddleware = func(req *http.Request) (*http.Response, error)

func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {
	var span trace.Span = nil
	defer func() {
		if span != nil {
			span.End()
		}
	}()

	reqData := requestData{
		url:    req.URL,
		header: req.Header,
		body:   nil,
	}

	// Intercept the request body so we can parse it and pass it along for real
	// processing.
	var err error
	reqData.body, req.Body, err = cloneBody(req.Body)
	if err != nil {
		return nil, err
	}

	var etracer endpointTracer = NOOP_ENDPOINT_TRACER

	// Start a span with data parsed from the request.
	switch req.URL.Path {
	case "/v1/responses":
		etracer = NewV1ResponsesTracer()
	}

	_, span, err = etracer.startSpanFromRequest(req.Context(), reqData)
	if err != nil {
		// Proceed if there's an error in tracing code
		log.Printf("Error starting span: %v", err)
	}

	// Continue processing the request.
	resp, err := next(req)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return resp, err
	}

	// Intercept the response body so we can extract data from it.
	var body []byte
	body, resp.Body, err = cloneBody(resp.Body)
	if err != nil {
		// I think we have to return an error here
		return resp, err
	}

	err = etracer.tagSpanWithResponse(span, body)
	if err != nil {
		// Proceed if there's an error in tracing code
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

type requestData struct {
	url    *url.URL
	body   []byte
	header http.Header
}

type endpointTracer interface {
	startSpanFromRequest(ctx context.Context, req requestData) (context.Context, trace.Span, error)
	tagSpanWithResponse(span trace.Span, body []byte) error
}

// noopTracer is an endpoint tracer that does nothing.
type noopTracer struct{}

func (*noopTracer) startSpanFromRequest(ctx context.Context, req requestData) (context.Context, trace.Span, error) {
	return ctx, nil, nil
}

func (*noopTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	return nil
}

var _ endpointTracer = &noopTracer{}
var NOOP_ENDPOINT_TRACER = &noopTracer{}
