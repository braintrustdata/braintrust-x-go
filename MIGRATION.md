# Migration Guide

This document provides guidance for migrating from `braintrust-x-go` to the new official Braintrust Go SDK: [braintrust-sdk-go](https://github.com/braintrustdata/braintrust-sdk-go).

## Why Migrate?

The new SDK offers a cleaner architecture with:
- Improved HTTP api patterns.
- Initialization / state improvements.
- Smaller more focused eval interface
- Improved type safety

## New Repository

- Repository: https://github.com/braintrustdata/braintrust-sdk-go
- Documentation: https://pkg.go.dev/github.com/braintrustdata/braintrust-sdk-go
- Examples: https://github.com/braintrustdata/braintrust-sdk-go/tree/main/examples

## Installation

```bash
go get github.com/braintrustdata/braintrust-sdk-go
```

## Major Changes

### 1. Client Initialization

**Old (braintrust-x-go):**
```go
import "github.com/braintrustdata/braintrust-x-go/braintrust/trace"

teardown, err := trace.Quickstart()
if err != nil {
    log.Fatal(err)
}
defer teardown()
```

**New (braintrust-sdk-go):**
```go
import (
    "github.com/braintrustdata/braintrust-sdk-go"
    "github.com/braintrustdata/braintrust-sdk-go/trace"
    "go.opentelemetry.io/otel"
)

tp := trace.NewTracerProvider()
defer tp.Shutdown(context.Background())
otel.SetTracerProvider(tp)

bt, err := braintrust.New(tp)
if err != nil {
    log.Fatal(err)
}
```

**Key changes:**
- Explicit `TracerProvider` setup required
- Returns a `*braintrust.Client` instead of a teardown function

### 2. Evaluator Pattern

**Old (braintrust-x-go):**
```go
import "github.com/braintrustdata/braintrust-x-go/braintrust/eval"

_, err = eval.Run(context.Background(), eval.Opts[string, string]{
    Project:    "my-project",
    Experiment: "experiment-name",
    Cases:      cases,
    Task:       taskFunc,
    Scorers:    scorers,
})
```

**New (braintrust-sdk-go):**
```go
import "github.com/braintrustdata/braintrust-sdk-go/eval"

evaluator := braintrust.NewEvaluator[string, string](bt)

_, err = evaluator.Run(context.Background(), eval.Opts[string, string]{
    Experiment: "experiment-name",
    Cases:      cases,
    Task:       eval.T(taskFunc), // Wrap simple task functions
    Scorers:    scorers,
})
```

**Key changes:**
- Create evaluator explicitly: `braintrust.NewEvaluator[Input, Output](client)`
- Project setting removed from `Opts` (set at client level)
- Wrap simple task functions with `eval.T()`

### 3. Scorer Signatures

**Old (braintrust-x-go):**
```go
eval.NewScorer("scorer_name", func(ctx context.Context, input, expected, result string, metadata eval.Metadata) (eval.Scores, error) {
    // Scorer logic
})
```

**New (braintrust-sdk-go):**
```go
eval.NewScorer("scorer_name", func(ctx context.Context, tr eval.TaskResult[string, string]) (eval.Scores, error) {
    input := tr.Input
    expected := tr.Expected
    result := tr.Output
    metadata := tr.Metadata
    // Scorer logic
})
```

**Key changes:**
- Scorers now receive a unified `TaskResult[I, O]` parameter
- Access fields via `taskResult.Input`, `taskResult.Expected`, `taskResult.Output`, `taskResult.Metadata`

### 4. LLM Integration Imports

**Old (braintrust-x-go):**
```go
import (
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceopenai"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/traceanthropic"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace/tracegenai"
)

client := openai.NewClient(
    option.WithMiddleware(traceopenai.Middleware),
)
```

**New (braintrust-sdk-go):**
```go
import (
    "github.com/braintrustdata/braintrust-sdk-go/trace/contrib/openai"
    "github.com/braintrustdata/braintrust-sdk-go/trace/contrib/anthropic"
    "github.com/braintrustdata/braintrust-sdk-go/trace/contrib/genai"
)

// OpenAI and Anthropic use middleware:
client := openai.NewClient(
    option.WithMiddleware(traceopenai.NewMiddleware()),
)

// Google Gemini uses HTTPClient wrapper:
client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
    HTTPClient: tracegenai.Client(),
    APIKey: os.Getenv("GOOGLE_API_KEY"),
    Backend: genai.BackendGeminiAPI,
})
```

**Key changes:**
- Import paths changed: `braintrust/trace/traceopenai` → `trace/contrib/openai`
- OpenAI/Anthropic: Middleware is now a function call `NewMiddleware()` instead of a variable
- Google Gemini: Uses `tracegenai.Client()` or `tracegenai.WrapClient()` for HTTPClient configuration

### 5. Hosted Resources (Tasks & Scorers)

**Old (braintrust-x-go):**
```go
import "github.com/braintrustdata/braintrust-x-go/braintrust/functions"

task, err := functions.GetTask(ctx, "task-slug")
scorer, err := functions.GetScorer(ctx, "scorer-slug")
```

**New (braintrust-sdk-go):**
```go
evaluator := braintrust.NewEvaluator[Input, Output](bt)

task, err := evaluator.Tasks().Get(ctx, "task-slug")
scorer, err := evaluator.Scorers().Get(ctx, "scorer-slug")
```

**Key changes:**
- Access via evaluator interface instead of standalone functions package

### 6. Dataset Handling

**Old (braintrust-x-go):**
```go
_, err = eval.Run(context.Background(), eval.Opts[I, O]{
    Project:      "my-project",
    DatasetID:    "dataset-id",
    DatasetLimit: 10,
    Task:         taskFunc,
    Scorers:      scorers,
})
```

**New (braintrust-sdk-go):**
```go
evaluator := braintrust.NewEvaluator[I, O](bt)

dataset, err := evaluator.Datasets().Get(ctx, "dataset-id")
if err != nil {
    log.Fatal(err)
}

_, err = evaluator.Run(context.Background(), eval.Opts[I, O]{
    Experiment: "experiment-name",
    Dataset:    Dataset,
    Task:       eval.T(taskFunc),
    Scorers:    scorers,
})
```

**Key changes:**
- No `DatasetID` field in evaluation options
- Pre-fetch dataset cases using `evaluator.Datasets().Get()`
- Pass fetched cases to `Run()`

### 7. Auto-Evaluations

**Old (braintrust-x-go):**
```go
import "github.com/braintrustdata/braintrust-x-go/braintrust/autoevals"

scorers := []eval.Scorer[I, O]{
    autoevals.EqualityScorer(),
}
```

**New (braintrust-sdk-go):**

The `autoevals` package was removed. Implement scorers inline:

```go
scorers := []eval.Scorer[I, O]{
    eval.NewScorer("exact_match", func(ctx context.Context, result eval.TaskResult[I, O]) (eval.Scores, error) {
        if result.Expected == result.Output {
            return eval.S(1.0), nil
        }
        return eval.S(0.0), nil
    }),
}
```

## Migration Checklist

- [ ] Update import paths from `braintrust-x-go` to `braintrust-sdk-go`
- [ ] Replace `trace.Quickstart()` with explicit `TracerProvider` setup
- [ ] Update client initialization with `braintrust.New()`
- [ ] Create evaluators using `braintrust.NewEvaluator[I, O](client)`
- [ ] Update scorer functions to accept `TaskResult[I, O]` parameter
- [ ] Update LLM integration imports to `trace/contrib/*` packages
- [ ] Change middleware usage from variables to `NewMiddleware()` functions
- [ ] Replace dataset ID references with `evaluator.Datasets().Get()` calls
- [ ] Remove `autoevals` usage and implement scorers inline
- [ ] Update hosted resource access via evaluator interface
- [ ] Test all changes thoroughly

## Additional Resources

- [New SDK Examples](https://github.com/braintrustdata/braintrust-sdk-go/tree/main/examples)
- [API Documentation](https://pkg.go.dev/github.com/braintrustdata/braintrust-sdk-go)
- [Braintrust Documentation](https://www.braintrust.dev/docs)

## Support

If you encounter issues during migration:
- Check the [examples directory](https://github.com/braintrustdata/braintrust-sdk-go/tree/main/examples) for working code
- Review the [API documentation](https://pkg.go.dev/github.com/braintrustdata/braintrust-sdk-go)
- Open an issue in the [new repository](https://github.com/braintrustdata/braintrust-sdk-go/issues)
