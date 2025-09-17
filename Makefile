.PHONY: help ci build clean test cover lint fmt mod-verify fix godoc examples

help:
	@echo "Available commands:"
	@echo "  help          - Show this help message"
	@echo "  build         - Build all packages"
	@echo "  test          - Run all tests"
	@echo "  cover         - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts and coverage files"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run golangci-lint"
	@echo "  fix           - Run golangci-lint with auto-fix"
	@echo "  godoc         - Start godoc server"
	@echo "  examples      - Run all examples"
	@echo "  ci            - Run CI pipeline (clean, lint, test, build)"
	@echo "  precommit     - Run fmt then ci"

ci: clean lint mod-verify test build

build:
	go build ./...

clean:
	go clean
	rm -rf coverage.out coverage.html dist

test:
	go test ./...

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint fmt -d
	golangci-lint run ./...

fmt:
	golangci-lint fmt

mod-verify:
	go mod tidy
	git diff --exit-code go.mod go.sum
	go mod verify

fix: fmt
	golangci-lint run --fix

godoc:
	@echo "Starting godoc server on http://localhost:6060"
	go run golang.org/x/tools/cmd/godoc@latest -http=:6060

examples:
	@echo "Running all examples..."
	@find examples -name "*.go" -exec sh -c 'echo "Running $$(dirname "{}")..." && cd "$$(dirname "{}")" && go run .' \;
	@echo "All examples completed!"

precommit: fmt ci
