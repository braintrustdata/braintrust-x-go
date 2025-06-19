# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build/Test Commands

- Build: `go build ./...`
- Run tests: `go test ./...`
- Run a single test: `go test -v -run=TestName ./path/to/package`
- Run tests with race detector: `go test -race ./...`
- Always run `make ci` to verify your changes
- Run `make fmt` to format code

## Code Style Guidelines

- Imports: Group standard library first, then external dependencies, then internal packages
- Error handling: Check all errors; use defer for cleanup; prefer explicit error returns
- Types: Use interfaces for abstraction; implement interface checks like `var _ Logger = (*NoopLogger)({})`
- Concurrency: Use mutexes for shared state; provide clear documentation for critical sections
- Documentation: All exported types/functions need comments; follow godoc formatting
- Testing: Use testify for assertions; create helpers for test setup/teardown; mock dependencies
- Naming: Use descriptive names; prefer shorter names for local variables