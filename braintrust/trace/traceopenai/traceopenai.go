// Package traceopenai provides OpenTelemetry middleware for tracing OpenAI API calls.
//
// First, set up tracing with Quickstart (requires BRAINTRUST_API_KEY environment variable):
//
//	// export BRAINTRUST_API_KEY="your-api-key-here"
//	teardown, err := trace.Quickstart()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer teardown()
//
// Then add the middleware to your OpenAI client:
//
//	client := openai.NewClient(
//		option.WithMiddleware(traceopenai.Middleware),
//	)
//
//	// Your OpenAI calls will now be automatically traced
//	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
//		Model: openai.F(openai.ChatModelGPT4),
//		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
//			openai.UserMessage("Hello!"),
//		}),
//	})
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

	"github.com/braintrust/braintrust-x-go/braintrust/diag"
)

func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

// NextMiddleware represents the next middleware to run in the OpenAI client middleware chain.
type NextMiddleware = func(req *http.Request) (*http.Response, error)

// Middleware adds OpenTelemetry tracing to OpenAI client requests.
// Ensure OpenTelemetry is properly configured before using this middleware.
func Middleware(req *http.Request, next NextMiddleware) (*http.Response, error) {
	diag.Debugf("Middleware: %s %s", req.Method, req.URL.Path)
	start := time.Now()

	// Intercept the request body so we can parse it and still pass it along.
	var buf bytes.Buffer
	reqBody := req.Body
	defer func() {
		if err := reqBody.Close(); err != nil {
			diag.Warnf("Error closing request body: %v", err)
		}
	}()
	tee := io.TeeReader(reqBody, &buf)
	req.Body = io.NopCloser(&buf)

	// Start a span with data parsed from the request.
	var reqTracer middlewareTracer = newNoopTracer()
	if req.URL != nil {
		switch req.URL.Path {
		case "/v1/responses":
			reqTracer = newResponsesTracer()
		case "/v1/chat/completions":
			reqTracer = newChatCompletionsTracer()
		}
	}

	ctx, span, err := reqTracer.StartSpan(req.Context(), start, tee)
	req = req.WithContext(ctx)
	if err != nil {
		diag.Warnf("Error starting span: %v", err)
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
			diag.Warnf("Error tagging span: %v\n%s", err)
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

func (*noopTracer) StartSpan(ctx context.Context, _ time.Time, _ io.Reader) (context.Context, trace.Span, error) {
	span := trace.SpanFromContext(context.Background()) // create a non-recording span
	return ctx, span, nil
}

func (*noopTracer) TagSpan(_ trace.Span, _ io.Reader) error {
	return nil
}

var _ middlewareTracer = &noopTracer{}
