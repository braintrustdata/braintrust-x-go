# Braintrust Go SDK v0.1 - TODO

**Overall Progress: ~45% Complete**

Last Updated: 2025-10-30

---

## Overview

This document tracks the remaining work to complete the v0.1 rewrite of the Braintrust Go SDK. The rewrite introduces a client-based architecture (`braintrust.New()`) to replace global state and improve testability.

### What's Been Completed ✅

- **Client Architecture** (`/client.go`, `/options.go`)
  - `braintrust.New()` creates configured client
  - Functional options pattern (WithAPIKey, WithProject, etc.)
  - Client lifecycle management (Shutdown)
  - Per-client configuration (no global state!)

- **Configuration Management** (`/config/config.go`)
  - Immutable Config struct
  - Environment variable loading
  - No global config cache (main fix!)
  - Test isolation support

- **Authentication/Session** (`/internal/auth/`)
  - Session-based async login with retry
  - Exponential backoff on 5xx/network errors
  - Fast failure on 4xx errors
  - Non-blocking Info() and blocking Login()
  - 10 comprehensive tests, all passing

- **Logger Interface** (`/logger/logger.go`, `/internal/tests/logger.go`)
  - Clean logger interface (Debug, Info, Warn, Error)
  - Default logger with BRAINTRUST_DEBUG support
  - Test utilities (NoopLogger, FailTestLogger)

- **Test Infrastructure**
  - Test loggers for different scenarios
  - README snippet compilation tests

- **Example Code** (`/examples/internal/rewrite/main.go`)
  - Working example of new braintrust.New() usage

- **Trace Integration** (`/client.go`, `/braintrust/trace/trace.go`) ✅
  - Client creates and manages TracerProvider
  - Trace setup integrated via client.setupTracing()
  - Braintrust exporter configured with session auth
  - Spans reach Braintrust successfully
  - Test verified end-to-end

### Critical Path

~~Trace Integration (DONE ✅)~~ → **Next: Eval Integration**

```
Trace Integration ✅ (DONE)
    ↓
Eval Integration (CURRENT - NO API CLIENT NEEDED!)
    ↓
API Client Refactoring (Can be done later)
    ↓
Documentation & Examples
```

**Key Insight:** Eval doesn't need the full API client refactor! The existing eval package already has its own API calls. We just need to pass the client's config/session to it.

---

## Phase 1: Core Functionality (HIGH PRIORITY)

### 1.1 Trace Integration ✅ COMPLETE

**Status:** Complete
**Complexity:** High

The Client now properly configures tracing with the Braintrust exporter.

**Completed Tasks:**
- [x] Refactor `braintrust/trace/trace.go` to work with Client
  - [x] Remove dependency on `braintrust.GetConfig()`
  - [x] Remove dependency on global auth cache
  - [x] Accept Client's TracerProvider instead of creating own
- [x] Create Braintrust OTLP exporter
  - [x] Use session.Info() for org/API URL
  - [x] Handle async login completion
  - [x] Configure proper auth headers
- [x] Implement span processor with filtering
  - [x] Apply FilterAISpans config
  - [x] Apply SpanFilterFuncs config
  - [x] Set parent span attributes (experiment ID, project ID)
- [x] Update Client.setupTracing()
  - [x] Call trace.Enable() with proper config
  - [x] Configure OTLP endpoint
  - [x] Attach span processor
- [x] Test end-to-end
  - [x] Verify spans reach Braintrust
  - [x] Test with async login
  - [x] Test with blocking login
  - [x] Test filtering works

**Files Modified:**
- `/client.go` - completed setupTracing()
- `/braintrust/trace/trace.go` - refactored to accept Client

---

### 1.2 Client Tests

**Status:** Not started
**Complexity:** Medium
**Priority:** High

No tests exist for the core Client type.

**Tasks:**
- [ ] Create `/client_test.go`
- [ ] Test Client creation
  - [ ] With default options
  - [ ] With custom options (all option types)
  - [ ] With invalid config (missing API key, etc.)
- [ ] Test multiple clients
  - [ ] Different configs in same process
  - [ ] Isolated auth sessions
  - [ ] Separate tracer providers
- [ ] Test lifecycle
  - [ ] Shutdown with owned provider
  - [ ] Shutdown with injected provider
  - [ ] Error cases
- [ ] Test blocking vs async login
  - [ ] Verify blocking login waits
  - [ ] Verify async login returns immediately
- [ ] Test String() method output
- [ ] Test logger integration

**Files to Create:**
- `/client_test.go`

---

### 1.3 API Client Refactoring

**Status:** Not started
**Complexity:** High
**Priority:** High
**Blocks:** Eval integration

The old API client (`/braintrust/api/`) uses global config. Need to create new namespaced API client.

**Current Old API:**
```go
// Old pattern - don't use
experiment.Get(experimentID)  // uses global config
```

**Desired New API:**
```go
// New pattern - want this
client.Projects.Get(ctx, projectID)
client.Projects.Register(ctx, name, opts)
client.Experiments.Get(ctx, experimentID)
client.Experiments.Register(ctx, name, projectID, opts)
client.Datasets.Get(ctx, datasetID)
client.Datasets.Query(ctx, datasetID, opts)
```

**Tasks:**
- [ ] Design API client structure
  - [ ] Decide on package location (api/ or internal/api/)
  - [ ] Define base Client struct with session, httpClient
  - [ ] Define namespaced sub-clients (Projects, Experiments, Datasets)
- [ ] Implement Projects client
  - [ ] Get(ctx, id) - get project by ID
  - [ ] Register(ctx, name, opts) - create/get project
  - [ ] List(ctx, opts) - list projects
  - [ ] Update(ctx, id, opts) - update project
  - [ ] Delete(ctx, id) - delete project
- [ ] Implement Experiments client
  - [ ] Get(ctx, id) - get experiment by ID
  - [ ] Register(ctx, name, projectID, opts) - create/get experiment
  - [ ] List(ctx, projectID, opts) - list experiments
  - [ ] Update(ctx, id, opts) - update experiment
  - [ ] Delete(ctx, id) - delete experiment
  - [ ] FetchResults(ctx, id, opts) - get experiment results
- [ ] Implement Datasets client
  - [ ] Get(ctx, id) - get dataset by ID
  - [ ] Register(ctx, name, opts) - create/get dataset
  - [ ] List(ctx, opts) - list datasets
  - [ ] Update(ctx, id, opts) - update dataset
  - [ ] Delete(ctx, id) - delete dataset
  - [ ] Query(ctx, id, opts) - query dataset records
  - [ ] Insert(ctx, id, records) - insert records
- [ ] Implement Prompts client
  - [ ] Get(ctx, id) - get prompt by ID
  - [ ] Register(ctx, name, opts) - create/get prompt
  - [ ] List(ctx, opts) - list prompts
- [ ] Add common infrastructure
  - [ ] Define request/response types
  - [ ] Add error handling patterns
  - [ ] Add retry logic (5xx, network errors)
  - [ ] Thread context.Context through all calls
  - [ ] Use session for authentication
  - [ ] Handle pagination
- [ ] Write tests
  - [ ] Mock server for each endpoint
  - [ ] Test success cases
  - [ ] Test error cases (4xx, 5xx, network)
  - [ ] Test retry logic
  - [ ] Test context cancellation
- [ ] Integrate with Client
  - [ ] Add API client fields to Client struct
  - [ ] Initialize in New()
  - [ ] Expose via Client methods or fields

**Files to Create:**
- `/api/client.go` - base API client
- `/api/projects.go` - Projects client
- `/api/experiments.go` - Experiments client
- `/api/datasets.go` - Datasets client
- `/api/prompts.go` - Prompts client
- `/api/types.go` - request/response types
- `/api/errors.go` - error handling
- `/api/*_test.go` - comprehensive tests

**Files to Modify:**
- `/client.go` - add API client fields, initialize in New()

**Migration Notes:**
- Old API at `/braintrust/api/` will be deprecated
- Keep old API during transition for backward compatibility
- Eventually remove old API in v0.2 or v1.0

---

## Phase 2: Eval Integration (HIGH PRIORITY - CURRENT)

**Status:** In Progress
**Complexity:** Medium
**Dependencies:** None! (eval has its own API calls)

Build a brand new eval package at `/eval` (not `/braintrust/eval/`) designed from the ground up to work with Client. No backward compatibility with old `eval.Run()`.

**Scope:**
- ✅ Cases with iterator interface
- ❌ No dataset loading initially (add later)
- ❌ No functions/prompts integration (add later)
- ❌ No parallel execution initially (add later)

**Tasks:**

### Phase 1: Write tests first (TDD)
- [ ] Write test for basic eval flow
  - [ ] Create client, call `client.Eval(ctx, opts)` with simple cases
  - [ ] Task function runs for each case
  - [ ] Scorers run and produce scores
  - [ ] Result contains scores, summary, permalink
  - [ ] Experiment is created in Braintrust
  - [ ] Spans are created with client's TracerProvider

- [ ] Write test for client integration
  - [ ] Eval uses client's project from config
  - [ ] Eval uses client's session for auth
  - [ ] Multiple clients stay isolated

- [ ] Write test for error cases
  - [ ] Missing experiment name
  - [ ] Empty cases
  - [ ] Task function returns error
  - [ ] Scorer returns error

### Phase 2: Build core types
- [ ] Create `/eval/types.go`
  - [ ] Define Opts[I, R] struct (Experiment, Cases, Task, Scorers, Metadata, Update)
  - [ ] Define Case[I, R] struct (Input, Expected, Tags, Metadata)
  - [ ] Define Cases[I, R] iterator interface
  - [ ] Define Task[I, R] function type
  - [ ] Define Scorer[I, R] interface
  - [ ] Define Result struct
- [ ] Create `/eval/cases.go`
  - [ ] Implement NewCases() helper for slice of cases
  - [ ] Implement iterator pattern

### Phase 3: Implement eval execution
- [ ] Create `/eval/eval.go`
  - [ ] Internal run function that accepts client resources (config, session, tracerProvider)
  - [ ] Iterate over cases, call task function
  - [ ] Run scorers for each case
  - [ ] Log results to experiment (spans + summaries)
  - [ ] Generate permalink
  - [ ] Return Result

- [ ] Create `/eval/experiment.go`
  - [ ] Register/get experiment using client's session
  - [ ] Log spans for each case
  - [ ] Set proper parent span attributes
  - [ ] Use client's TracerProvider

- [ ] Create `/eval/scorers.go`
  - [ ] Implement ExactMatch scorer

### Phase 4: Wire up Client
- [ ] Add to `/client.go`
  ```go
  func (c *Client) Eval[I, R any](ctx context.Context, opts eval.Opts[I, R]) (*eval.Result, error) {
      return eval.Run(ctx, opts, c.config, c.session, c.tracerProvider)
  }
  ```

- [ ] Write integration test
  - [ ] End-to-end test with real client
  - [ ] Verify experiment appears in Braintrust
  - [ ] Verify spans appear in Braintrust

### Phase 5: Example and verification
- [ ] Create `/examples/eval/main.go` - simple working example
- [ ] Run tests and verify
  - [ ] `make test` passes
  - [ ] `make ci` passes
  - [ ] Example runs successfully

### Phase 6: Validate API compatibility
- [ ] Add task to TODO.md: "Figure out eval hooks / validate compatibility with Python API"
  - [ ] Verify hooks API matches Python
  - [ ] Verify metadata handling matches Python
  - [ ] Verify span attributes match Python

**Files to Create:**
- `/eval/types.go` - Core types (Opts, Case, Cases interface, Task, Scorer, Result)
- `/eval/cases.go` - Cases iterator implementation and helpers
- `/eval/eval.go` - Execution engine
- `/eval/experiment.go` - Experiment API
- `/eval/scorers.go` - ExactMatch scorer
- `/eval/eval_test.go` - Comprehensive tests
- `/examples/eval/main.go` - Working example

**Files to Modify:**
- `/client.go` - add Eval[I, R]() method

**Example Usage:**
```go
client := braintrust.New(braintrust.WithProject("my-project"))
defer client.Shutdown(ctx)

result, err := client.Eval(ctx, eval.Opts[string, string]{
    Experiment: "greeting-test",
    Cases: eval.NewCases([]eval.Case[string, string]{
        {Input: "World", Expected: "Hello, World!"},
        {Input: "Alice", Expected: "Hello, Alice!"},
    }),
    Task: func(ctx context.Context, input string) (string, error) {
        return "Hello, " + input + "!", nil
    },
    Scorers: []eval.Scorer[string, string]{
        eval.ExactMatch[string, string](),
    },
})
```

**Estimated effort:** 1 day (focused scope, TDD)

**⚠️ NOTE:** PICKUP AFTER REBASE WITH CASE.TAGS CHANGES

---

## Phase 3: Documentation & Migration (MEDIUM PRIORITY)

**Status:** Not started
**Complexity:** Medium
**Priority:** Medium

Update docs and examples to reflect new Client-based API.

**Tasks:**
- [ ] Update README.md
  - [ ] Replace trace.Quickstart() example with braintrust.New()
  - [ ] Show new eval pattern with Client.Eval()
  - [ ] Show new API client usage
  - [ ] Add migration notes section
- [ ] Update package doc.go
  - [ ] Explain Client-based architecture
  - [ ] Show quick start with braintrust.New()
  - [ ] Explain options pattern
- [ ] Create migration guide
  - [ ] Document breaking changes
  - [ ] Show side-by-side comparisons (old vs new)
  - [ ] Provide migration checklist
  - [ ] Explain timeline for deprecation
- [ ] Update all examples
  - [ ] `/examples/anthropic/` - use braintrust.New()
  - [ ] `/examples/openai/` - use braintrust.New()
  - [ ] `/examples/genai/` - use braintrust.New()
  - [ ] `/examples/evals/` - use Client.Eval()
  - [ ] `/examples/datasets/` - use new API client
  - [ ] `/examples/prompts/` - use new API client
  - [ ] All other examples
- [ ] Add comprehensive godoc comments
  - [ ] Document all exported types
  - [ ] Document all exported functions
  - [ ] Add examples in godoc
  - [ ] Document options and their defaults

**Files to Update:**
- `/README.md`
- `/doc.go`
- Create `/MIGRATION.md`
- All files in `/examples/`

---

## Phase 4: Cleanup (MEDIUM PRIORITY)

**Status:** Not started
**Complexity:** Low-Medium
**Priority:** Medium

Deprecate and eventually remove old global config pattern.

**Tasks:**
- [ ] Mark deprecated
  - [ ] Add deprecation comment to braintrust.GetConfig()
  - [ ] Add deprecation comment to trace.Quickstart()
  - [ ] Add deprecation comment to old API functions
  - [ ] Update godoc to point to new patterns
- [ ] Create deprecation timeline
  - [ ] v0.1: New API available, old API deprecated
  - [ ] v0.2: Old API removed?
  - [ ] v1.0: Clean slate with only new API?
- [ ] Update tests to avoid deprecated APIs
- [ ] Eventually remove (decide on version)
  - [ ] `/braintrust/env.go` - remove GetConfig()
  - [ ] `/braintrust/login.go` - remove global login
  - [ ] Old API implementations
  - [ ] Old trace.Quickstart() if not needed

**Files to Modify:**
- `/braintrust/env.go` - add deprecation
- `/braintrust/login.go` - add deprecation
- `/braintrust/trace/trace.go` - maybe add deprecation to Quickstart()

**Migration Strategy:**
- Keep both APIs working during transition
- Give users time to migrate
- Remove old API in major version bump

---

## Phase 5: Future-Proofing (LOW PRIORITY)

**Status:** Not started
**Complexity:** Low
**Priority:** Low

Add extension points for future features without implementing them.

**Tasks:**
- [ ] Define extension point interfaces
  ```go
  // Reserved for future use - do not implement yet
  type EvalHooks interface {
      Metadata() Metadata
      SetMetadata(key string, val any)
      Tags() []string
      AddTag(tag string)
      Expected() R
      TrialIndex() int
      Span() trace.Span
  }

  type ErrorScoreFunc func(error) Scores
  ```
- [ ] Add reserved fields to eval.Opts
  ```go
  type Opts struct {
      // ...existing fields...

      // Reserved for future use
      TrialCount        int           // Future: multiple trials per case
      ErrorScoreHandler ErrorScoreFunc // Future: custom error scoring
      BaseExperiment    string        // Future: comparison experiments
      Timeout           time.Duration // Future: per-task timeout
      NoSendLogs        bool          // Future: local-only mode
  }
  ```
- [ ] Document as "reserved for future use"
  - [ ] Add godoc comments explaining they're placeholders
  - [ ] Note that they will be implemented in future versions
  - [ ] Prevent API breakage when actually implemented

**Files to Modify:**
- `/braintrust/eval/eval.go` - add reserved fields
- Create `/future.go` - define extension interfaces

---

## API Section

### Current State

The old API implementation exists at `/braintrust/api/` with these issues:
- Uses `braintrust.GetConfig()` (global state)
- No context.Context threading
- Not namespaced under client
- Limited error handling
- No retry logic

### Desired Client-Based API

We want a namespaced API client that's part of the main Client:

```go
// Initialize client
bt, _ := braintrust.New(
    braintrust.WithAPIKey(apiKey),
    braintrust.WithProject("my-project"),
)
defer bt.Shutdown(ctx)

// Use namespaced API
project, _ := bt.API.Projects.Register(ctx, "my-project", nil)
exp, _ := bt.API.Experiments.Register(ctx, "my-exp", project.ID, nil)
dataset, _ := bt.API.Datasets.Get(ctx, datasetID)
records, _ := bt.API.Datasets.Query(ctx, datasetID, QueryOpts{Limit: 100})
```

### API Client Structure

```go
type APIClient struct {
    session    *auth.Session
    httpClient *http.Client

    Projects    *ProjectsClient
    Experiments *ExperimentsClient
    Datasets    *DatasetsClient
    Prompts     *PromptsClient
}

type ProjectsClient struct {
    client *APIClient
}

func (c *ProjectsClient) Get(ctx context.Context, id string) (*Project, error)
func (c *ProjectsClient) Register(ctx context.Context, name string, opts *ProjectOpts) (*Project, error)
func (c *ProjectsClient) List(ctx context.Context, opts *ListOpts) ([]*Project, error)
func (c *ProjectsClient) Update(ctx context.Context, id string, opts *ProjectUpdateOpts) (*Project, error)
func (c *ProjectsClient) Delete(ctx context.Context, id string) error
```

### API Implementation Checklist

**Infrastructure:**
- [ ] Create `/api/client.go` with base APIClient
- [ ] Add HTTP client with proper User-Agent
- [ ] Add request builder (method, path, body, auth)
- [ ] Add response parser (JSON decode, error handling)
- [ ] Add retry logic (exponential backoff on 5xx)
- [ ] Add pagination support
- [ ] Add rate limiting handling
- [ ] Thread context.Context through all calls

**Projects Client:**
- [ ] Implement Get - GET /v1/project/:id
- [ ] Implement Register - POST /v1/project
- [ ] Implement List - GET /v1/project
- [ ] Implement Update - PATCH /v1/project/:id
- [ ] Implement Delete - DELETE /v1/project/:id
- [ ] Define Project type
- [ ] Define ProjectOpts type
- [ ] Write tests with mock server

**Experiments Client:**
- [ ] Implement Get - GET /v1/experiment/:id
- [ ] Implement Register - POST /v1/experiment
- [ ] Implement List - GET /v1/experiment
- [ ] Implement Update - PATCH /v1/experiment/:id
- [ ] Implement Delete - DELETE /v1/experiment/:id
- [ ] Implement FetchResults - GET /v1/experiment/:id/results
- [ ] Define Experiment type
- [ ] Define ExperimentOpts type
- [ ] Define ExperimentResults type
- [ ] Write tests with mock server

**Datasets Client:**
- [ ] Implement Get - GET /v1/dataset/:id
- [ ] Implement Register - POST /v1/dataset
- [ ] Implement List - GET /v1/dataset
- [ ] Implement Update - PATCH /v1/dataset/:id
- [ ] Implement Delete - DELETE /v1/dataset/:id
- [ ] Implement Query - GET /v1/dataset/:id/query
- [ ] Implement Insert - POST /v1/dataset/:id/insert
- [ ] Define Dataset type
- [ ] Define DatasetOpts type
- [ ] Define QueryOpts type
- [ ] Define DatasetRecord type
- [ ] Write tests with mock server

**Prompts Client:**
- [ ] Implement Get - GET /v1/prompt/:id
- [ ] Implement Register - POST /v1/prompt
- [ ] Implement List - GET /v1/prompt
- [ ] Define Prompt type
- [ ] Define PromptOpts type
- [ ] Write tests with mock server

**Error Handling:**
- [ ] Define APIError type
- [ ] Parse error responses from Braintrust API
- [ ] Wrap network errors appropriately
- [ ] Handle rate limiting (429 responses)
- [ ] Handle auth errors (401/403)
- [ ] Handle server errors (5xx with retry)

**Testing:**
- [ ] Create test utilities for mock HTTP server
- [ ] Test each endpoint's success case
- [ ] Test error responses (4xx, 5xx)
- [ ] Test retry logic on 5xx
- [ ] Test context cancellation
- [ ] Test authentication header
- [ ] Test pagination
- [ ] Integration test with real API (optional)

### Migration from Old API

**Old Pattern:**
```go
import "github.com/braintrustdata/braintrust-x-go/braintrust/api/experiment"

exp, err := experiment.Get(experimentID)
```

**New Pattern:**
```go
bt, _ := braintrust.New(braintrust.WithAPIKey(key))
defer bt.Shutdown(ctx)

exp, err := bt.API.Experiments.Get(ctx, experimentID)
```

**Benefits:**
- No global state
- Context cancellation support
- Better error handling
- Retry logic built-in
- Multiple clients possible
- Testable in isolation

---

## Summary

**Critical Path:**
1. Trace Integration (blocks everything) - ~2-3 days
2. API Client Refactoring - ~3-4 days
3. Eval Integration - ~1-2 days
4. Documentation & Examples - ~1-2 days

**Estimated Time to v0.1:** ~1-2 weeks of focused work

**Key Decisions Needed:**
- When to deprecate old API? (suggest v0.1 deprecate, v0.2 remove)
- Should eval.Run() signature change or keep backward compat?
- Should trace.Quickstart() still work or force migration?
- API client location: `/api/` or `/internal/api/`?

**Success Criteria:**
- [ ] Client works end-to-end with tracing
- [ ] All examples updated and working
- [ ] API client fully functional
- [ ] Eval works with new Client
- [ ] All tests passing
- [ ] Documentation complete
- [ ] Migration guide available
