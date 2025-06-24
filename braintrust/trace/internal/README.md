# Internal Trace Package

This package provides shared utilities and middleware functionality for OpenTelemetry tracers across different AI providers (OpenAI, Anthropic, etc.).
## Components

### Shared Utilities (`utils.go`)

- **`BufferedReader`**: Handles response body buffering with completion callbacks
- **`ParseUsageTokens`**: Normalizes token usage metrics across different API formats
- **`SetJSONAttr`**: Helper for setting JSON attributes on OpenTelemetry spans

### Shared Middleware (`middleware.go`)

- **`Middleware`**: Generic HTTP middleware factory for tracing API requests
- **`MiddlewareTracer`**: Interface for provider-specific endpoint tracers
- **`TracerRouter`**: Function type for mapping URL paths to tracers
- **`NoopTracer`**: Default tracer for unsupported endpoints
- **`GetTracer`**: Returns the shared "braintrust" OpenTelemetry tracer
