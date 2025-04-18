package traceopenai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("braintrust")

type NextMiddleware = func(req *http.Request) (*http.Response, error)

func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {

	rd := requestData{
		url:    req.URL,
		header: req.Header,
	}

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		rd.body = bodyBytes
	}

	var span trace.Span
	var err error

	switch req.URL.Path {
	case "/v1/responses":
		span, err = startSpanFromV1ResponseRequest(req.Context(), rd)
		if err != nil {
			fmt.Println("FIXME: error", err)
		}
	}

	resp, err := next(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	span.End()

	return resp, nil
}

type requestData struct {
	url    *url.URL
	body   []byte
	header http.Header
}
