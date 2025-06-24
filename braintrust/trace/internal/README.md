# Internal Shared Utilities

This package provides shared middleware functionality for OpenTelemetry tracers used across different AI providers (OpenAI, Anthropic, etc.).

## Shared Components

- **`BufferedReader`**: Handles response body buffering with completion callbacks
- **`ToInt64`**: Converts various numeric types to int64 for metric values
- **`SetJSONAttr`**: Helper for setting JSON attributes on OpenTelemetry spans

## Middleware Architecture

The shared middleware uses a router pattern to direct requests to appropriate tracers:

```go
// Each provider implements a router function
func providerRouter(path string) MiddlewareTracer {
    switch path {
    case "/v1/specific/endpoint":
        return newSpecificTracer()
    default:
        return internal.NewNoopTracer()
    }
}

// Middleware is created using the router
var Middleware = internal.Middleware(providerRouter)
```

This architecture makes it easy to add new AI providers by implementing the `MiddlewareTracer` interface and creating a router function.

## Usage Token Parsing

Each tracer implements its own `ParseUsageTokens` function to handle provider-specific token metrics:
- **OpenAI**: Standard token parsing with detailed breakdowns and metric prefix translation
- **Anthropic**: Enhanced parsing with cache token handling (cache_creation_input_tokens, cache_read_input_tokens)

Both use the shared `ToInt64` utility function for consistent numeric conversions.
