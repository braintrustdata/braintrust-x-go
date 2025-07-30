
# Braintrust Go Tracing & Eval SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/braintrustdata/braintrust-x-go.svg)](https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go)

This SDK is currently is in BETA status and APIs may change.

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

    // Define your AI function to evaluate
    greetingTask := func(ctx context.Context, input string) (string, error) {
        return "Hello " + input, nil
    }

    // Define scoring function
    exactMatch := func(ctx context.Context, input, expected, result string, _ eval.Metadata) (eval.Scores, error) {
        if expected == result {
            return eval.S(1.0), nil // Perfect match
        }
        return eval.S(0.0), nil // No match
    }

    // Create an evaluation
    experimentID, err := eval.ResolveProjectExperimentID("greeting-experiment", "my-project")
    if err != nil {
        log.Fatal(err)
    }

    evaluation := eval.New(experimentID,
        eval.NewCases([]eval.Case[string, string]{
            {Input: "World", Expected: "Hello World"},
            {Input: "Alice", Expected: "Hello Alice"},
        }),
        greetingTask,
        []eval.Scorer[string, string]{
            eval.NewScorer("exact_match", exactMatch),
        },
    )

    // Run the evaluation
    err = evaluation.Run(context.Background())
    if err != nil {
        log.Fatal(err)
    }
}
```

### OpenAI Tracing

```go
package main

import (
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
    // ...
}
```

### Anthropic Tracing

```go
package main

import (
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
    // ...
}
```

## Features

- **Evaluations**: Run systematic evaluations of your AI systems with custom scoring functions
- **Tracing**: Automatic instrumentation for OpenAI and Anthropic API calls
- **Datasets**: Manage and version your evaluation datasets
- **Experiments**: Track different versions and configurations of your AI systems
- **Observability**: Monitor your AI applications in production

## Examples

Check out the [`examples/`](./examples/) directory for complete working examples:

- [`examples/evals/`](./examples/evals/) - Basic evaluation setup
- [`examples/traceopenai/`](./examples/traceopenai/) - OpenAI tracing
- [`examples/anthropic-tracer/`](./examples/anthropic-tracer/) - Anthropic tracing
- [`examples/dataset-eval/`](./examples/dataset-eval/) - Dataset-based evaluations

## Documentation

- [Braintrust Documentation](https://www.braintrust.dev/docs)
- [API Reference](https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and contribution guidelines.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](./LICENSE) file for details.
