# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build/Test Commands

- Build: `go build ./...`
- Run tests: `go test ./...`
- Run a single test: `go test -v -run=TestName ./path/to/package`
- Run tests with race detector: `go test -race ./...`
- Run all tests with `make test`
- Always run `make ci` to verify your changes (tests, lint, fmt, etc)

## Development process

- Follow test driven development. Write failing tests before implementing
  features.
