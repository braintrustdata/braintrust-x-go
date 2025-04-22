package traceopenai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("braintrust")

type NextMiddleware = func(req *http.Request) (*http.Response, error)

func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {
	var span trace.Span = nil
	defer func() {
		if span != nil {
			span.End()
		}
	}()

	var err error
	reqData := requestData{
		url:    req.URL,
		header: req.Header,
		body:   nil,
	}

	// Intercept the request body so we can parse it and pass it along for real
	// processing.
	reqData.body, req.Body, err = cloneBody(req.Body)
	if err != nil {
		return nil, err
	}

	var etracer endpointTracer = &noopTracer{}

	// Start a span with data parsed from the request.
	switch req.URL.Path {
	case "/v1/responses":
		etracer = NewV1ResponsesTracer()
	}

	span, err = etracer.startSpanFromRequest(req.Context(), reqData)

	// Continue processing the request.
	resp, err := next(req)
	if err != nil && span != nil {
		span.RecordError(err)
	}

	// Intercept the response body so we can extract data from it.
	var body []byte
	body, resp.Body, err = cloneBody(resp.Body)
	if err != nil {
		return resp, err
	}

	err = etracer.tagSpanWithResponse(span, body)
	if err != nil {
		return resp, err
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
	startSpanFromRequest(ctx context.Context, req requestData) (trace.Span, error)
	tagSpanWithResponse(span trace.Span, body []byte) error
}

// noopTracer is an endpoint tracer that does nothing.
type noopTracer struct{}

func (t *noopTracer) startSpanFromRequest(ctx context.Context, req requestData) (trace.Span, error) {
	return nil, nil
}

func (t *noopTracer) tagSpanWithResponse(span trace.Span, body []byte) error {
	return nil
}

var _ endpointTracer = &noopTracer{}
