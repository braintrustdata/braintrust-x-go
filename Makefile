.PHONY: help ci build clean test cover lint fmt mod-tidy mod-verify

help:
	@echo "Available commands:"
	@echo "  help          - Show this help message"
	@echo "  build         - Build all packages"
	@echo "  test          - Run all tests"
	@echo "  cover         - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts and coverage files"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run golangci-lint"
	@echo "  mod-tidy      - Tidy and verify Go modules"
	@echo "  ci            - Run CI pipeline (clean, lint, test, build)"

ci: clean lint mod-verify test build

build:
	go build ./...

clean:
	go clean
	rm -f coverage.out coverage.html

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

mod-tidy:
	go mod tidy
