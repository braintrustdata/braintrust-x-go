
# Braintrust Go SDK

This SDK is currently is in BETA status and APIs may change.

The official Go SDK for [Braintrust](https://www.braintrust.dev), a platform for building reliable AI applications through evaluation, experimentation, and observability.

## Installation

```bash
go get github.com/braintrust/braintrust-x-go
```

## Quick Start

### Set up your API key

```bash
export BRAINTRUST_API_KEY="your-api-key"
```

### Basic Evaluation

```go
package main

import (
    "context"
    "log"
    
    "github.com/braintrust/braintrust-x-go/braintrust/eval"
)

func main() {
    // Define your AI function to evaluate
    myAIFunction := func(ctx context.Context, input string) (string, error) {
        // Your AI logic here
        return "some result", nil
    }

    // Create an evaluation
    experimentID, err := eval.ResolveProjectExperimentID("my-experiment", "my-project")
    if err != nil {
        log.Fatal(err)
    }

    evaluation := eval.New(experimentID,
        eval.NewCases([]eval.Case[string, string]{
            {Input: "test input", Expected: "expected output"},
        }),
        myAIFunction,
        []eval.Scorer[string, string]{
            // Add your scoring functions here
        },
    )

    // Run the evaluation
    err = evaluation.Run(context.Background())
    if err != nil {
        log.Fatal(err)
    }
}
```

### AI Model Tracing

Automatically trace your OpenAI and Anthropic API calls:

```go
package main

import (
    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
    
    "github.com/braintrust/braintrust-x-go/braintrust/trace"
    "github.com/braintrust/braintrust-x-go/braintrust/trace/traceopenai"
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

    // Your API calls will now be automatically traced
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
- [API Reference](https://pkg.go.dev/github.com/braintrust/braintrust-x-go)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and contribution guidelines.

## License

This project is licensed under the MIT License.
