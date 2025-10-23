// Package tracelangchaingo provides OpenTelemetry tracing for LangChainGo applications.
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
// Then create a handler and add it to your LangChainGo LLM:
//
//	handler := tracelangchaingo.NewHandler()
//	llm, err := openai.New(openai.WithCallback(handler))
//
//	// Your LangChainGo calls will now be automatically traced
//	resp, err := llm.GenerateContent(ctx, []llms.MessageContent{
//		llms.TextParts(llms.ChatMessageTypeHuman, "Hello!"),
//	})
package tracelangchaingo

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"

	"github.com/braintrustdata/braintrust-x-go/braintrust/trace/internal"
)

// Handler implements the LangChainGo callbacks.Handler interface to provide
// OpenTelemetry tracing for LangChainGo applications.
type Handler struct {
	mu sync.RWMutex
	// We need to track spans because we can't return the modified context from callbacks.
	// We use the original context as the key to look up the span later.
	// This works because LangChainGo passes the same context to Start/End/Error callbacks.
	spans map[context.Context]trace.Span
}

// NewHandler creates a new Handler for tracing LangChainGo operations.
func NewHandler() *Handler {
	return &Handler{
		spans: make(map[context.Context]trace.Span),
	}
}

// tracer returns the shared braintrust tracer.
func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer("braintrust")
}

// HandleLLMStart is called at the start of an LLM call with simple string prompts.
func (h *Handler) HandleLLMStart(ctx context.Context, prompts []string) {
	_, span := tracer().Start(ctx, "langchain.llm.call")

	// Store prompts as input
	if err := internal.SetJSONAttr(span, "braintrust.input_json", prompts); err != nil {
		span.RecordError(err)
	}

	// Store span for later retrieval
	h.mu.Lock()
	h.spans[ctx] = span
	h.mu.Unlock()
}

// HandleLLMGenerateContentStart is called at the start of a GenerateContent call.
func (h *Handler) HandleLLMGenerateContentStart(ctx context.Context, ms []llms.MessageContent) {
	_, span := tracer().Start(ctx, "langchain.llm.generate_content")

	// Convert messages to a format suitable for tracing
	messages := make([]map[string]interface{}, len(ms))
	for i, m := range ms {
		messages[i] = map[string]interface{}{
			"role":  string(m.Role),
			"parts": m.Parts,
		}
	}

	if err := internal.SetJSONAttr(span, "braintrust.input_json", messages); err != nil {
		span.RecordError(err)
	}

	// Store span for later retrieval
	h.mu.Lock()
	h.spans[ctx] = span
	h.mu.Unlock()
}

// HandleLLMGenerateContentEnd is called when a GenerateContent call completes.
func (h *Handler) HandleLLMGenerateContentEnd(ctx context.Context, res *llms.ContentResponse) {
	// Retrieve the span from our map
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		// No active span for this context
		return
	}

	// Extract output from response
	if res != nil && len(res.Choices) > 0 {
		choices := make([]map[string]interface{}, len(res.Choices))
		for i, choice := range res.Choices {
			choiceMap := map[string]interface{}{
				"content":     choice.Content,
				"stop_reason": choice.StopReason,
			}
			if choice.FuncCall != nil {
				choiceMap["function_call"] = choice.FuncCall
			}
			if len(choice.ToolCalls) > 0 {
				choiceMap["tool_calls"] = choice.ToolCalls
			}
			if choice.GenerationInfo != nil {
				choiceMap["generation_info"] = choice.GenerationInfo
			}
			choices[i] = choiceMap
		}

		if err := internal.SetJSONAttr(span, "braintrust.output_json", choices); err != nil {
			span.RecordError(err)
		}

		// Extract metrics from generation info if available
		if len(res.Choices) > 0 && res.Choices[0].GenerationInfo != nil {
			h.extractMetrics(span, res.Choices[0].GenerationInfo)
		}
	}

	span.End()
}

// HandleLLMError is called when an LLM call results in an error.
func (h *Handler) HandleLLMError(ctx context.Context, err error) {
	// Retrieve the span from our map
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		// No active span for this context
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.End()
}

// extractMetrics attempts to extract token usage metrics from generation info
func (h *Handler) extractMetrics(span trace.Span, genInfo map[string]any) {
	metrics := make(map[string]int64)

	// Look for common token usage fields
	if usage, ok := genInfo["usage"].(map[string]any); ok {
		for k, v := range usage {
			if ok, i := internal.ToInt64(v); ok {
				switch k {
				case "prompt_tokens":
					metrics["prompt_tokens"] = i
				case "completion_tokens":
					metrics["completion_tokens"] = i
				case "total_tokens":
					metrics["tokens"] = i
				default:
					metrics[k] = i
				}
			}
		}
	}

	if len(metrics) > 0 {
		if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
			span.RecordError(err)
		}
	}
}

// Chain callbacks

// HandleChainStart is called at the start of a chain execution.
func (h *Handler) HandleChainStart(ctx context.Context, inputs map[string]any) {
	_, span := tracer().Start(ctx, "langchain.chain")

	if err := internal.SetJSONAttr(span, "braintrust.input_json", inputs); err != nil {
		span.RecordError(err)
	}

	// Store span for later retrieval
	h.mu.Lock()
	h.spans[ctx] = span
	h.mu.Unlock()
}

// HandleChainEnd is called when a chain execution completes.
func (h *Handler) HandleChainEnd(ctx context.Context, outputs map[string]any) {
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		return
	}

	if err := internal.SetJSONAttr(span, "braintrust.output_json", outputs); err != nil {
		span.RecordError(err)
	}

	span.End()
}

// HandleChainError is called when a chain execution results in an error.
func (h *Handler) HandleChainError(ctx context.Context, err error) {
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.End()
}

// Tool callbacks

// HandleToolStart is called at the start of a tool execution.
func (h *Handler) HandleToolStart(ctx context.Context, input string) {
	_, span := tracer().Start(ctx, "langchain.tool")

	if err := internal.SetJSONAttr(span, "braintrust.input_json", input); err != nil {
		span.RecordError(err)
	}

	// Store span for later retrieval
	h.mu.Lock()
	h.spans[ctx] = span
	h.mu.Unlock()
}

// HandleToolEnd is called when a tool execution completes.
func (h *Handler) HandleToolEnd(ctx context.Context, output string) {
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		return
	}

	if err := internal.SetJSONAttr(span, "braintrust.output_json", output); err != nil {
		span.RecordError(err)
	}

	span.End()
}

// HandleToolError is called when a tool execution results in an error.
func (h *Handler) HandleToolError(ctx context.Context, err error) {
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.End()
}

// Agent callbacks

// HandleAgentAction is called when an agent performs an action.
func (h *Handler) HandleAgentAction(ctx context.Context, action schema.AgentAction) {
	_, span := tracer().Start(ctx, "langchain.agent.action")

	actionData := map[string]interface{}{
		"tool":       action.Tool,
		"tool_input": action.ToolInput,
		"log":        action.Log,
	}

	if err := internal.SetJSONAttr(span, "braintrust.input_json", actionData); err != nil {
		span.RecordError(err)
	}

	// Agent actions are instantaneous events, so end immediately
	span.End()
}

// HandleAgentFinish is called when an agent finishes.
func (h *Handler) HandleAgentFinish(ctx context.Context, finish schema.AgentFinish) {
	_, span := tracer().Start(ctx, "langchain.agent.finish")

	finishData := map[string]interface{}{
		"return_values": finish.ReturnValues,
		"log":           finish.Log,
	}

	if err := internal.SetJSONAttr(span, "braintrust.output_json", finishData); err != nil {
		span.RecordError(err)
	}

	// Agent finish is an instantaneous event, so end immediately
	span.End()
}

// Retriever callbacks

// HandleRetrieverStart is called at the start of a retrieval operation.
func (h *Handler) HandleRetrieverStart(ctx context.Context, query string) {
	_, span := tracer().Start(ctx, "langchain.retriever")

	if err := internal.SetJSONAttr(span, "braintrust.input_json", query); err != nil {
		span.RecordError(err)
	}

	// Store span for later retrieval
	h.mu.Lock()
	h.spans[ctx] = span
	h.mu.Unlock()
}

// HandleRetrieverEnd is called when a retrieval operation completes.
func (h *Handler) HandleRetrieverEnd(ctx context.Context, query string, documents []schema.Document) {
	h.mu.Lock()
	span, ok := h.spans[ctx]
	delete(h.spans, ctx)
	h.mu.Unlock()

	if !ok {
		return
	}

	// Convert documents to a simple format for tracing
	docs := make([]map[string]interface{}, len(documents))
	for i, doc := range documents {
		docs[i] = map[string]interface{}{
			"page_content": doc.PageContent,
			"metadata":     doc.Metadata,
		}
	}

	output := map[string]interface{}{
		"query":     query,
		"documents": docs,
		"count":     len(documents),
	}

	if err := internal.SetJSONAttr(span, "braintrust.output_json", output); err != nil {
		span.RecordError(err)
	}

	// Add document count as a metric
	metrics := map[string]int64{
		"documents_retrieved": int64(len(documents)),
	}
	if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
		span.RecordError(err)
	}

	span.End()
}

// Utility callbacks

// HandleText is called to handle text events.
// This creates an instantaneous event span to record the text.
func (h *Handler) HandleText(ctx context.Context, text string) {
	_, span := tracer().Start(ctx, "langchain.text")

	if err := internal.SetJSONAttr(span, "braintrust.input_json", text); err != nil {
		span.RecordError(err)
	}

	// Text events are instantaneous, so end immediately
	span.End()
}

// HandleStreamingFunc is called to handle streaming chunks.
// This creates an instantaneous event span for each chunk.
func (h *Handler) HandleStreamingFunc(ctx context.Context, chunk []byte) {
	_, span := tracer().Start(ctx, "langchain.streaming.chunk")

	if err := internal.SetJSONAttr(span, "braintrust.input_json", string(chunk)); err != nil {
		span.RecordError(err)
	}

	// Add chunk size as a metric
	metrics := map[string]int64{
		"chunk_size": int64(len(chunk)),
	}
	if err := internal.SetJSONAttr(span, "braintrust.metrics", metrics); err != nil {
		span.RecordError(err)
	}

	// Streaming chunks are instantaneous events, so end immediately
	span.End()
}

// Ensure Handler implements callbacks.Handler
var _ interface {
	HandleText(ctx context.Context, text string)
	HandleLLMStart(ctx context.Context, prompts []string)
	HandleLLMGenerateContentStart(ctx context.Context, ms []llms.MessageContent)
	HandleLLMGenerateContentEnd(ctx context.Context, res *llms.ContentResponse)
	HandleLLMError(ctx context.Context, err error)
	HandleChainStart(ctx context.Context, inputs map[string]any)
	HandleChainEnd(ctx context.Context, outputs map[string]any)
	HandleChainError(ctx context.Context, err error)
	HandleToolStart(ctx context.Context, input string)
	HandleToolEnd(ctx context.Context, output string)
	HandleToolError(ctx context.Context, err error)
	HandleAgentAction(ctx context.Context, action schema.AgentAction)
	HandleAgentFinish(ctx context.Context, finish schema.AgentFinish)
	HandleRetrieverStart(ctx context.Context, query string)
	HandleRetrieverEnd(ctx context.Context, query string, documents []schema.Document)
	HandleStreamingFunc(ctx context.Context, chunk []byte)
} = (*Handler)(nil)
