.PHONY: help ci build examples clean test cover lint fmt tidy dev

# Default target
help:
	@echo "Available commands:"
	@echo "  help          - Show this help message"
	@echo "  build         - Build all packages"
	@echo "  test          - Run all tests"
	@echo "  cover         - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts and coverage files"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run golangci-lint"
	@echo "  tidy          - Tidy and verify Go modules"
	@echo "  examples      - Run all example programs"
	@echo "  ci            - Run CI pipeline (clean, lint, test, build)"
	@echo "  dev           - Run development pipeline (ci + examples)"

# Verify the build for ci.
ci: clean lint test build

build:
	go build ./...

# Run all of the examples.
examples:
	go run ./examples/evals
	go run ./examples/traceopenai

clean:
	go clean
	rm -f coverage.out coverage.html

test:
	go test ./...

# Run tests with coverage
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -html=coverage.out

lint:
	golangci-lint fmt -d
	golangci-lint run ./...

fmt:
	golangci-lint fmt


# Tidy and verify dependencies
tidy:
	go mod tidy
	go mod verify

# Verify the build and run the examples.
dev: ci examples
