# Braintrust Dev Server API Documentation

## Overview

The Braintrust Dev Server is a Go library that enables remote execution of AI evaluators from the Braintrust web interface. Users import the library, register their evaluators programmatically, and start a local HTTP server.

**Key Features:**
- Run evaluators defined in local code from the web playground
- Stream evaluation progress in real-time via Server-Sent Events
- Use custom scoring functions alongside built-in evaluators
- Execute evaluations against datasets stored in Braintrust or provided inline

**Default Configuration:**
- Host: `localhost`
- Port: `8300`
- Protocol: HTTP

### Usage Pattern

Users create a `main.go` file that imports the devserver package and registers evaluators:

```go
package main

import (
    "context"
    "log"
    "strings"

    "github.com/braintrustdata/braintrust-x-go/braintrust/devserver"
    "github.com/braintrustdata/braintrust-x-go/braintrust/eval"
    "github.com/braintrustdata/braintrust-x-go/braintrust/trace"
)

func main() {
    // Initialize tracing
    teardown, err := trace.Quickstart()
    if err != nil {
        log.Fatalf("Failed to initialize tracing: %v", err)
    }
    defer teardown()

    // Create dev server
    server := devserver.New(devserver.Config{
        Host:    "localhost",
        Port:    8300,
        OrgName: "", // Optional: restrict to specific org
    })

    // Register evaluators
    err = devserver.Register(server, devserver.RemoteEval[string, string]{
        Name:        "uppercase",
        ProjectName: "my-project",
        Task: func(ctx context.Context, input string) (string, error) {
            return strings.ToUpper(input), nil
        },
        Scorers: []eval.Scorer[string, string]{
            eval.NewScorer("length", func(ctx context.Context, input, expected, result string, meta eval.Metadata) (eval.Scores, error) {
                score := float64(len(result)) / 10.0
                if score > 1.0 {
                    score = 1.0
                }
                return eval.S(score), nil
            }),
        },
    })
    if err != nil {
        log.Fatalf("Failed to register evaluator: %v", err)
    }

    // Start server (blocks)
    log.Println("Starting dev server on http://localhost:8300")
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

Then run it:
```bash
go run main.go
```

The server will be accessible from the Braintrust web interface for remote evaluation execution.

---

## Architecture

### High-Level Flow

```
┌─────────────────┐          ┌──────────────────┐          ┌─────────────────┐
│  Braintrust Web │  HTTPS   │  Local Dev       │  Local   │  Evaluators     │
│  Interface      │─────────▶│  Server          │─────────▶│  (User Code)    │
│  (Playground)   │◀─────────│  (localhost:8300)│◀─────────│                 │
└─────────────────┘   SSE    └──────────────────┘          └─────────────────┘
                                     │
                                     │ API Calls
                                     ▼
                              ┌──────────────────┐
                              │  Braintrust API  │
                              │  (for datasets,  │
                              │  scorers, etc)   │
                              └──────────────────┘
```

### Request Processing Pipeline

```
HTTP Request
    │
    ▼
┌─────────────────────────────────────────────┐
│ 1. CORS Middleware                          │
│    - Validate origin                        │
│    - Set CORS headers                       │
│    - Handle preflight requests              │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 2. Authorization Middleware                 │
│    - Extract token from headers             │
│    - Create request context                 │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 3. CheckAuthorized Middleware               │
│    (only for /list and /eval)               │
│    - Validate token with Braintrust API     │
│    - Cache login state (LRU cache)          │
│    - Check org restriction if configured    │
└─────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ 4. Route Handler                            │
│    - Process request                        │
│    - Execute evaluator                      │
│    - Return response (JSON or SSE)          │
└─────────────────────────────────────────────┘
```

---

## API Endpoints

### GET `/`

**Description:** Health check endpoint

**Authentication:** None required

**Response:**
```
200 OK
Content-Type: text/plain

Hello, world!
```

---

### GET `/list`

**Description:** List all available evaluators and their metadata

**Authentication:** Required (Bearer token or x-bt-auth-token header)

**Response:**
```json
{
  "evaluator_name": {
    "parameters": {
      "param_name": {
        "type": "prompt" | "data",
        "description": "string (optional)",
        "default": "any (optional)",
        "schema": {} // JSON Schema (only for type: "data")
      }
    },
    "scores": [
      { "name": "score_name" }
    ]
  }
}
```

**Example Response:**
```json
{
  "summarization_eval": {
    "parameters": {
      "model": {
        "type": "data",
        "description": "The model to use for summarization",
        "schema": {
          "type": "string",
          "enum": ["gpt-4", "gpt-3.5-turbo"]
        },
        "default": "gpt-4"
      },
      "prompt_template": {
        "type": "prompt",
        "description": "Template for summarization",
        "default": {
          "model": "gpt-4",
          "messages": [
            {"role": "system", "content": "You are a helpful assistant"}
          ]
        }
      }
    },
    "scores": [
      { "name": "accuracy" },
      { "name": "coherence" }
    ]
  }
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - Missing or invalid authentication token

---

### POST `/eval`

**Description:** Execute an evaluator with provided data

**Authentication:** Required (Bearer token or x-bt-auth-token header)

**Content-Type:** `application/json`

**Request Body Schema:**
```typescript
{
  name: string;                    // Required: evaluator name
  parameters?: Record<string, any>; // Optional: parameter overrides
  data: {                          // Required: dataset specification
    // Option 1: By project/dataset name
    project_name: string;
    dataset_name: string;
    _internal_btql?: string;       // Optional BTQL filter

    // OR Option 2: By dataset ID
    dataset_id: string;
    _internal_btql?: string;

    // OR Option 3: Inline data
    data: Array<{
      input: any;
      expected?: any;
      metadata?: Record<string, any>;
    }>;
  };
  scores?: Array<{                 // Optional: additional remote scorers
    name: string;
    function_id: {
      // One of:
      function_id?: string;
      version?: string;
      // OR
      name?: string;
      // OR
      prompt_session_id?: string;
      // OR
      inline_code?: string;
      // OR
      global_function?: string;
    };
  }>;
  experiment_name?: string;        // Optional: experiment name override
  project_id?: string;             // Optional: project ID override
  parent?: string | {              // Optional: parent span for tracing
    object_type: string;
    object_id: string;
  };
  stream?: boolean;                // Optional: enable SSE streaming (default: false)
}
```

**Example Request:**
```json
{
  "name": "summarization_eval",
  "parameters": {
    "model": "gpt-4"
  },
  "data": {
    "project_name": "my-project",
    "dataset_name": "test-dataset"
  },
  "experiment_name": "run-2024-01-15",
  "stream": true,
  "scores": [
    {
      "name": "custom_scorer",
      "function_id": {
        "function_id": "func_abc123"
      }
    }
  ]
}
```

**Response (Non-Streaming):**

Content-Type: `application/json`

```json
{
  "experimentName": "string",
  "projectName": "string",
  "projectId": "string",
  "experimentId": "string",
  "experimentUrl": "string",
  "projectUrl": "string",
  "comparisonExperimentName": "string | null",
  "scores": {
    "score_name": {
      "name": "string",
      "score": "number",
      "improvements": "number",
      "regressions": "number"
    }
  }
}
```

**Response (Streaming):**

Content-Type: `text/event-stream`

See [Server-Sent Events](#server-sent-events-sse-streaming) section for details.

**Status Codes:**
- `200 OK` - Success (JSON response)
- `200 OK` - Success (SSE stream)
- `400 Bad Request` - Invalid request body, parameters, or dataset
- `401 Unauthorized` - Missing or invalid authentication token
- `404 Not Found` - Evaluator not found
- `500 Internal Server Error` - Server error during evaluation

---

## Authentication & Authorization

### Token Extraction

The server accepts authentication tokens in three formats (in order of precedence):

1. **Custom header (preferred):**
   ```
   x-bt-auth-token: <token>
   ```

2. **Bearer token:**
   ```
   Authorization: Bearer <token>
   ```

3. **Direct token:**
   ```
   Authorization: <token>
   ```

### Authorization Flow

```
1. Extract token from request headers
   │
   ▼
2. Check LRU cache for existing login state
   │
   ├─ Cache hit? ──▶ Return cached BraintrustState
   │
   └─ Cache miss? ──▶ Call loginToState(token, orgName)
                       │
                       ▼
                   3. Validate token with Braintrust API
                       │
                       ├─ Valid? ──▶ Create BraintrustState
                       │              │
                       │              ▼
                       │          Cache state (max 32 entries)
                       │              │
                       │              ▼
                       │          Return BraintrustState
                       │
                       └─ Invalid? ─▶ Return 401 Unauthorized
```

### Request Context

Each authenticated request has a context object attached:

```typescript
interface RequestContext {
  appOrigin: string;           // Validated origin from CORS
  token: string | undefined;   // Extracted auth token
  state: BraintrustState | undefined; // Cached login state
}
```

### Organization Restriction

When `OrgName` is specified in the server config, the server validates that the authenticated user belongs to that organization. This is useful for:
- Multi-tenant environments
- Security restrictions
- Team-specific evaluators

---

## CORS Configuration

### Allowed Origins

The server whitelists the following origins:

1. **Primary origin:** `https://www.braintrust.dev`
2. **Alternative origin:** `https://www.braintrustdata.com`
3. **Preview environments:** `https://*.preview.braintrust.dev` (regex pattern)
4. **Environment variables:**
   - `WHITELISTED_ORIGIN` - Additional custom origin
   - `BRAINTRUST_APP_URL` - App-specific origin

### Origin Validation

```go
func checkOrigin(requestOrigin string) bool {
    if requestOrigin == "" {
        return true // Allow requests without origin (e.g., same-origin)
    }

    for _, allowedOrigin := range whitelistedOrigins {
        if matchOrigin(allowedOrigin, requestOrigin) {
            return true
        }
    }

    return false
}
```

### CORS Headers

**Preflight Response (OPTIONS):**
```
Access-Control-Allow-Origin: <validated-origin>
Access-Control-Allow-Methods: GET, PATCH, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: <see below>
Access-Control-Allow-Credentials: true
Access-Control-Max-Age: 86400
Access-Control-Allow-Private-Network: true (if requested)
```

**Actual Response:**
```
Access-Control-Allow-Origin: <validated-origin>
Access-Control-Allow-Credentials: true
Access-Control-Expose-Headers: x-bt-cursor, x-bt-found-existing-experiment, x-bt-span-id, x-bt-span-export
```

### Allowed Headers

```
Content-Type
X-Amz-Date
Authorization
X-Api-Key
X-Amz-Security-Token
x-bt-auth-token
x-bt-parent
x-bt-org-name
x-bt-stream-fmt
x-bt-use-cache
x-stainless-os
x-stainless-lang
x-stainless-package-version
x-stainless-runtime
x-stainless-runtime-version
x-stainless-arch
```

### Private Network Access

The server supports Chrome's Private Network Access feature:

**Request:**
```
Access-Control-Request-Private-Network: true
```

**Response:**
```
Access-Control-Allow-Private-Network: true
```

This allows the web app (on public internet) to make requests to localhost.

---

## Request/Response Schemas

### Parameter Types

#### Prompt Parameter
```typescript
{
  type: "prompt",
  description?: string,
  default?: {
    model?: string,
    messages?: Array<{
      role: "system" | "user" | "assistant",
      content: string
    }>,
    temperature?: number,
    max_tokens?: number,
    // ... other prompt fields
  }
}
```

#### Data Parameter
```typescript
{
  type: "data",
  description?: string,
  schema: object, // JSON Schema
  default?: any
}
```

Example JSON Schema for data parameter:
```json
{
  "type": "string",
  "enum": ["gpt-4", "gpt-3.5-turbo", "claude-2"],
  "default": "gpt-4"
}
```

### Function ID Specification

Function IDs can be specified in multiple ways:

```typescript
type FunctionId =
  | { function_id: string; version?: string }
  | { name: string }
  | { prompt_session_id: string }
  | { inline_code: string }
  | { global_function: string };
```

**Examples:**
```json
// By function ID
{ "function_id": "func_abc123", "version": "v2" }

// By name
{ "name": "my_scorer_function" }

// By prompt session
{ "prompt_session_id": "session_xyz789" }

// Inline code
{ "inline_code": "def score(input, output, expected): return {'score': 1.0}" }

// Global function
{ "global_function": "built_in_scorer" }
```

### Dataset Specification

Three ways to specify dataset:

#### 1. By Project and Dataset Name
```json
{
  "project_name": "my-project",
  "dataset_name": "test-dataset",
  "_internal_btql": "SELECT * WHERE difficulty = 'hard'" // optional filter
}
```

#### 2. By Dataset ID
```json
{
  "dataset_id": "ds_abc123",
  "_internal_btql": "SELECT * LIMIT 100"
}
```

#### 3. Inline Data
```json
{
  "data": [
    {
      "input": "What is the capital of France?",
      "expected": "Paris",
      "metadata": { "difficulty": "easy" }
    },
    {
      "input": "What is the capital of Japan?",
      "expected": "Tokyo",
      "metadata": { "difficulty": "easy" }
    }
  ]
}
```

### Evaluation Summary Response

```typescript
{
  experimentName: string;
  projectName: string;
  projectId: string;
  experimentId: string;
  experimentUrl: string;
  projectUrl: string;
  comparisonExperimentName: string | null;
  scores: {
    [scoreName: string]: {
      name: string;
      score: number;           // Average score across all cases
      improvements: number;    // Count of improvements vs baseline
      regressions: number;     // Count of regressions vs baseline
    }
  }
}
```

---

## Server-Sent Events (SSE) Streaming

### Overview

When `stream: true` is specified in the eval request, the server responds with a Server-Sent Events stream instead of a single JSON response.

**Content-Type:** `text/event-stream`

**Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

### Event Format

SSE events follow this format:
```
event: <event-type>
data: <json-payload>

```

Note: Each event ends with two newlines (`\n\n`).

### Event Types

#### 1. `start` Event

Sent immediately when the evaluation begins.

```
event: start
data: {"experimentName":"run-001","projectId":"proj_123","projectName":"my-project"}

```

**Payload:**
```typescript
{
  experimentName: string;
  projectId: string;
  projectName: string;
}
```

#### 2. `progress` Event

Sent continuously during evaluation execution. Contains individual task outputs and scoring results.

```
event: progress
data: {"format":"code","output_type":"completion","event":"json_delta","data":"{\"result\":\"Summary of the text\"}"}

```

**Payload:**
```typescript
{
  format: "code" | "text" | "image";
  output_type: "completion" | "score" | "metadata";
  event: "json_delta" | "text_delta" | "image";
  data: string; // JSON-encoded or raw string
}
```

**Common Progress Events:**
- Task outputs (JSON completions)
- Intermediate scores
- Metadata updates
- Error reports

#### 3. `summary` Event

Sent once after all evaluations complete, before the `done` event.

```
event: summary
data: {"experimentName":"run-001","scores":{"accuracy":{"name":"accuracy","score":0.85,"improvements":5,"regressions":2}}}

```

**Payload:** Same as non-streaming response (see [Evaluation Summary Response](#evaluation-summary-response))

#### 4. `done` Event

Sent at the very end to signal stream completion.

```
event: done
data:

```

**Payload:** Empty string

#### 5. `error` Event

Sent if an error occurs during evaluation.

```
event: error
data: "Error message describing what went wrong"

```

After an error event, the stream is closed.

### Example SSE Stream

```
event: start
data: {"experimentName":"sentiment-analysis-001","projectId":"proj_abc123","projectName":"sentiment-project"}

event: progress
data: {"format":"code","output_type":"completion","event":"json_delta","data":"{\"sentiment\":\"positive\",\"confidence\":0.92}"}

event: progress
data: {"format":"code","output_type":"completion","event":"json_delta","data":"{\"sentiment\":\"negative\",\"confidence\":0.78}"}

event: progress
data: {"format":"code","output_type":"score","event":"json_delta","data":"{\"accuracy\":0.85}"}

event: summary
data: {"experimentName":"sentiment-analysis-001","projectName":"sentiment-project","projectId":"proj_abc123","experimentId":"exp_xyz789","experimentUrl":"https://www.braintrust.dev/app/proj_abc123/exp_xyz789","projectUrl":"https://www.braintrust.dev/app/proj_abc123","comparisonExperimentName":null,"scores":{"accuracy":{"name":"accuracy","score":0.85,"improvements":15,"regressions":5}}}

event: done
data:

```

### Client-Side Handling

Client code (JavaScript) can consume the stream:

```javascript
const eventSource = new EventSource('/eval', {
  method: 'POST',
  headers: { 'x-bt-auth-token': token },
  body: JSON.stringify(requestBody)
});

eventSource.addEventListener('start', (e) => {
  const data = JSON.parse(e.data);
  console.log('Started:', data.experimentName);
});

eventSource.addEventListener('progress', (e) => {
  const data = JSON.parse(e.data);
  console.log('Progress:', data);
});

eventSource.addEventListener('summary', (e) => {
  const data = JSON.parse(e.data);
  console.log('Summary:', data);
});

eventSource.addEventListener('done', (e) => {
  console.log('Stream complete');
  eventSource.close();
});

eventSource.addEventListener('error', (e) => {
  console.error('Error:', e.data);
  eventSource.close();
});
```

---

## Dataset Handling

### Loading Strategy

The server supports three dataset loading strategies:

#### 1. By Project and Dataset Name

**Flow:**
```
1. Receive: { project_name, dataset_name, _internal_btql? }
   │
   ▼
2. Call: initDataset(state, project, dataset, btql)
   │
   ▼
3. SDK fetches dataset from Braintrust API
   │
   ▼
4. Apply BTQL filter if provided
   │
   ▼
5. Return: EvalData array
```

#### 2. By Dataset ID

**Flow:**
```
1. Receive: { dataset_id, _internal_btql? }
   │
   ▼
2. Call: API endpoint to get dataset metadata
   │
   ▼
3. Extract: projectId and dataset name
   │
   ▼
4. Call: initDataset(state, projectId, datasetName, btql)
   │
   ▼
5. Return: EvalData array
```

**API Call:**
```http
POST /api/dataset/get
Content-Type: application/json

{"id": "ds_abc123"}
```

**Response:**
```json
[
  {
    "project_id": "proj_xyz",
    "name": "my-dataset"
  }
]
```

#### 3. Inline Data

**Flow:**
```
1. Receive: { data: [...] }
   │
   ▼
2. Validate: Array of eval cases
   │
   ▼
3. Return: Data array directly (no API calls)
```

### Eval Case Format

Each evaluation case has this structure:

```typescript
interface EvalCase<Input, Expected, Metadata> {
  input: Input;           // Required: input to the evaluator task
  expected?: Expected;    // Optional: expected output for comparison
  metadata?: Metadata;    // Optional: additional metadata
  tags?: string[];        // Optional: tags for filtering/grouping
}
```

**Example:**
```json
{
  "input": {
    "text": "This product is amazing!",
    "language": "en"
  },
  "expected": {
    "sentiment": "positive",
    "confidence": 0.9
  },
  "metadata": {
    "source": "reviews",
    "product_id": "prod_123"
  },
  "tags": ["high-confidence", "english"]
}
```

---

## Parameter Validation

### Validation Flow

```
1. Client sends parameters in eval request
   │
   ▼
2. Server looks up evaluator definition
   │
   ▼
3. Check if evaluator.parameters exists
   │
   ├─ No parameters? ──▶ Skip validation
   │
   └─ Has parameters? ─▶ Validate each parameter
                          │
                          ├─ Type: "prompt"
                          │   └─▶ Validate against PromptData schema
                          │
                          └─ Type: "data"
                              └─▶ Validate against JSON Schema
                                  │
                                  ├─ Valid? ──▶ Pass to evaluator
                                  │
                                  └─ Invalid? ─▶ Return 400 Bad Request
```

### Prompt Parameter Validation

Prompt parameters must conform to the Braintrust PromptData schema:

```typescript
interface PromptData {
  model?: string;
  messages?: Array<{
    role: "system" | "user" | "assistant" | "function";
    content?: string;
    name?: string;
    function_call?: object;
  }>;
  temperature?: number;
  max_tokens?: number;
  top_p?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
  tools?: Array<object>;
  tool_choice?: string | object;
  // ... additional fields
}
```

### Data Parameter Validation

Data parameters are validated using JSON Schema. Common validations:

**String with enum:**
```json
{
  "type": "string",
  "enum": ["option1", "option2", "option3"]
}
```

**Number with range:**
```json
{
  "type": "number",
  "minimum": 0,
  "maximum": 1
}
```

**Object with properties:**
```json
{
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "number"}
  },
  "required": ["name"]
}
```

**Array:**
```json
{
  "type": "array",
  "items": {"type": "string"},
  "minItems": 1
}
```

### Error Responses

**Missing required parameter:**
```json
{
  "error": "Parameter 'model' is required"
}
```

**Type mismatch:**
```json
{
  "error": "Parameter 'temperature' must be a number, got string"
}
```

**Validation failure:**
```json
{
  "error": "Parameter 'model' must be one of: gpt-4, gpt-3.5-turbo"
}
```

---

## Remote Scorer Integration

### Overview

Remote scorers allow the web interface to specify additional scoring functions that should be applied during evaluation. These scorers are defined in Braintrust (as Functions) and invoked via API calls.

### Scorer Specification

Scorers are provided in the eval request:

```json
{
  "scores": [
    {
      "name": "custom_relevance",
      "function_id": {
        "function_id": "func_abc123"
      }
    },
    {
      "name": "hallucination_check",
      "function_id": {
        "name": "hallucination_scorer"
      }
    }
  ]
}
```

### Scorer Proxy Function

The server creates a proxy function for each remote scorer:

```typescript
function makeScorer(
  state: BraintrustState,
  name: string,
  functionId: FunctionId
): EvalScorer {
  return async (input, output, expected, metadata) => {
    // Build invoke request
    const request = {
      ...functionId,
      input: {
        input,
        output,
        expected,
        metadata
      },
      parent: await getSpanParentObject().export(),
      stream: false,
      mode: "auto",
      strict: true
    };

    // Call Braintrust function invoke API
    const response = await state.proxyConn().post(
      'function/invoke',
      request,
      { headers: { Accept: 'application/json' } }
    );

    // Parse and return scores
    return await response.json();
  };
}
```

### Function Invoke API

**Endpoint:** `POST https://api.braintrust.dev/function/invoke`

**Headers:**
```
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/json
```

**Request:**
```json
{
  "function_id": "func_abc123",
  "version": "v1",
  "input": {
    "input": "What is AI?",
    "output": "Artificial Intelligence is...",
    "expected": "AI stands for...",
    "metadata": {}
  },
  "parent": {
    "object_type": "project_logs",
    "object_id": "span_xyz789",
    "row_ids": [...]
  },
  "stream": false,
  "mode": "auto",
  "strict": true
}
```

**Response:**
```json
{
  "score": 0.85,
  "name": "relevance",
  "metadata": {
    "reasoning": "The answer is relevant and accurate"
  }
}
```

Or multiple scores:
```json
[
  {
    "score": 0.85,
    "name": "relevance"
  },
  {
    "score": 0.92,
    "name": "accuracy"
  }
]
```

### Score Aggregation

The evaluation framework combines:
1. Built-in evaluator scores
2. Remote scorer results

All scores are aggregated in the final summary with statistics (mean, improvements, regressions).
