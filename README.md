
# Braintrust Go Tracing & Eval SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/braintrustdata/braintrust-x-go.svg)](https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go)
![Beta](https://img.shields.io/badge/status-beta-yellow)

## Overview

This library provides tools for **evaluating** and **tracing** AI applications in [Braintrust](https://www.braintrust.dev). Use it to:

- **Evaluate** your AI models with custom test cases and scoring functions
- **Trace** LLM calls and monitor AI application performance with OpenTelemetry
- **Integrate** seamlessly with OpenAI, Anthropic, Google Gemini, LangChainGo, and other LLM providers

This SDK is currently in BETA status and APIs may change.

## Installation

```bash
go get github.com/braintrustdata/braintrust-x-go
```

## Quick Start

### Set up your API key

```bash
export BRAINTRUST_API_KEY="your-api-key"
```

### Evals

```go
package main

import (
    "context"
    "log"
    
    "github.com/braintrustdata/braintrust-x-go/braintrust/eval"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
    // Set up tracing
    teardown, err := trace.Quickstart()
    if err != nil {
        log.Fatal(err)
    }
    defer teardown()

    // Run an evaluation
    _, err = eval.Run(context.Background(), eval.Opts[string, string]{
        Project:    "my-project",
        Experiment: "greeting-experiment",
        Cases: eval.NewCases([]eval.Case[string, string]{
            {Input: "World", Expected: "Hello World"},
            {Input: "Alice", Expected: "Hello Alice"},
        }),
        Task: func(ctx context.Context, input string) (string, error) {
            return "Hello " + input, nil
        },
        Scorers: []eval.Scorer[string, string]{
            eval.NewScorer("exact_match", func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
                if expected == result {
                    return eval.S(1.0), nil
                }
                return eval.S(0.0), nil
            }),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### OpenAI Tracing

```go
package main

import (
    "log"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"

    "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
)

func main() {
    // Start tracing
    teardown, err := trace.Quickstart()
    if err != nil {
        log.Fatal(err)
    }
    defer teardown()

    // Create OpenAI client with tracing middleware
    client := openai.NewClient(
        option.WithMiddleware(traceopenai.Middleware),
    )

    // Your OpenAI API calls will now be automatically traced
    _ = client // Use the client for your API calls
}
```

### Anthropic Tracing

```go
package main

import (
    "log"

    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"

    "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceanthropic"
)

func main() {
    // Start tracing
    teardown, err := trace.Quickstart()
    if err != nil {
        log.Fatal(err)
    }
    defer teardown()

    // Create Anthropic client with tracing middleware
    client := anthropic.NewClient(
        option.WithMiddleware(traceanthropic.Middleware),
    )

    // Your Anthropic API calls will now be automatically traced
    _ = client // Use the client for your API calls
}
```

### Google Gemini Tracing

```go
package main

import (
    "context"
    "log"
    "os"

    "google.golang.org/genai"

    "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracegenai"
)

func main() {
    // Start tracing
    teardown, err := trace.Quickstart()
    if err != nil {
        log.Fatal(err)
    }
    defer teardown()

    // Create Gemini client with tracing
    client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
        HTTPClient: tracegenai.Client(),
        APIKey:     os.Getenv("GOOGLE_API_KEY"),
        Backend:    genai.BackendGeminiAPI,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Your Gemini API calls will now be automatically traced
    _ = client // Use the client for your API calls
}
```

### LangChainGo Integration

The SDK provides comprehensive tracing support for [LangChainGo](https://github.com/tmc/langchaingo) applications. Automatically trace LLM calls, chains, tools, agents, and retrievers by passing the Braintrust callback handler to your LangChainGo components. See [`examples/langchaingo`](./examples/langchaingo/main.go) for a complete example.

## Features

- **Evaluations**: Run systematic evaluations of your AI systems with custom scoring functions
- **Tracing**: Automatic instrumentation for OpenAI, Anthropic, Google Gemini, and LangChainGo
- **Datasets**: Manage and version your evaluation datasets
- **Experiments**: Track different versions and configurations of your AI systems
- **Observability**: Monitor your AI applications in production

## Examples

Check out the [`examples/`](./examples/) directory for complete working examples:

- [evals](./examples/evals/evals.go) - Create and run evaluations with custom test cases and scoring functions
- [openai](./examples/openai/main.go) - Automatically trace OpenAI API calls
- [anthropic](./examples/anthropic/main.go) - Automatically trace Anthropic API calls
- [genai](./examples/genai/main.go) - Automatically trace Google Gemini API calls
- [langchaingo](./examples/langchaingo/main.go) - Trace LangChainGo applications (chains, tools, agents, retrievers)
- [datasets](./examples/datasets/main.go) - Run evaluations using datasets stored in Braintrust

## Documentation

- [Braintrust Documentation](https://www.braintrust.dev/docs)
- [API Reference](https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and contribution guidelines.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](./LICENSE) file for details.
