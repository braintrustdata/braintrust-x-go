# Anthropic Tracer

This package provides OpenTelemetry middleware for tracing Anthropic API calls, specifically the Messages endpoint. The tracer automatically captures request and response data, including input messages, output content, usage metrics, and metadata.

## Quick Start

### 1. Set up tracing

First, set up OpenTelemetry tracing with Braintrust:

```go
import (
    "github.com/braintrust/braintrust-x-go/braintrust/api"
    "github.com/braintrust/braintrust-x-go/braintrust"
    "github.com/braintrust/braintrust-x-go/braintrust/trace"
)

// Initialize braintrust tracing with a specific project
projectName := "my-anthropic-project"
project, err := api.RegisterProject(projectName)
if err != nil {
    log.Fatal(err)
}

opt := braintrust.WithDefaultProjectID(project.ID)
teardown, err := trace.Quickstart(opt)
if err != nil {
    log.Fatal(err)
}
defer teardown()
```

### 2. Add middleware to your Anthropic client

```go
import (
    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
    "github.com/braintrust/braintrust-x-go/braintrust/trace/traceanthropic"
)

client := anthropic.NewClient(
    option.WithAPIKey("your-api-key-here"),
    option.WithMiddleware(traceanthropic.Middleware),
)
```

### 3. Make API calls as usual

```go
message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model: anthropic.ModelClaude3_7SonnetLatest,
    Messages: []anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, Claude!")),
    },
    MaxTokens: 1024,
})
```

All your API calls will now be automatically traced and sent to Braintrust!

## Supported Endpoints

Currently supports:
- `/v1/messages` - The primary Messages endpoint for Claude conversations

## Streaming Support

The tracer automatically detects streaming requests and handles them appropriately:
Streaming responses are aggregated into a single trace with the complete conversation.

## Examples

See the [example](../../../examples/anthropic-tracer/) for a complete demonstration including:

## Environment Variables

- `BRAINTRUST_API_KEY`: Required for sending traces to Braintrust
- `ANTHROPIC_API_KEY`: Required for Anthropic API calls (handled by the Anthropic SDK)

## Requirements

- Go 1.18+
- OpenTelemetry configured (handled by `trace.Quickstart()`)
- Anthropic API key
- Braintrust API key (for trace collection)
