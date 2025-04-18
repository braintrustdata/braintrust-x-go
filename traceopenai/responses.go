package traceopenai

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type v1ResponseRequest struct {
	Model       string  `json:"model"`
	Input       string  `json:"input"`
	Temperature float64 `json:"temperature"`
}

func startSpanFromV1ResponseRequest(ctx context.Context, req requestData) (trace.Span, error) {
	_, span := tracer.Start(ctx, "openai.chat.completion")

	var v1Req v1ResponseRequest
	err := json.Unmarshal(req.body, &v1Req)
	if err != nil {
		return span, err
	}

	span.SetAttributes(attribute.String("model", v1Req.Model))
	span.SetAttributes(attribute.String("input", v1Req.Input))
	span.SetAttributes(attribute.Float64("temperature", v1Req.Temperature))

	return span, nil
}
