.PHONY: all build clean test cover lint fmt vet tidy run help

MODULE_NAME := github.com/braintrust/braintrust-x-go

all: fmt lint vet test build

build:
	go build .

run:
	go run .

clean:
	go clean

test:
	go test ./... -v

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
	go mod verify

# Build and run in one command
dev: fmt test run

# Print help message
help:
	@echo "Available targets:"
	@echo "  all       - Format, lint, vet, test, and build"
	@echo "  build     - Build the application"
	@echo "  run       - Run the application"
	@echo "  clean     - Remove build artifacts"
	@echo "  test      - Run tests"
	@echo "  cover     - Run tests with coverage report"
	@echo "  lint      - Run golangci-lint"
	@echo "  fmt       - Format code with go fmt"
	@echo "  vet       - Run go vet"
	@echo "  tidy      - Tidy and verify dependencies"
	@echo "  dev       - Format, test and run the application"
	@echo "  help      - Print this help message"

# Default target
.DEFAULT_GOAL := help
