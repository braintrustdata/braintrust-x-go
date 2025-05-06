
# Verify the build for ci.
ci: clean fmt lint vet test build

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
    go tool cover -html=coverage.out

lint:
    golangci-lint run ./...

fmt:
    go fmt ./...

vet:
    go vet ./...

# Tidy and verify dependencies
tidy:
    go mod tidy
    go mod verify

# Verify the build and run the examples.
dev: ci examples

# Run tests in the current directory when files change.
[no-cd]
watch-test-cwd:
    watchexec -c -r -w . -e go -- go test ./... -v

# Run all tests when files change.
watch-test:
    watchexec -c -r -w . -e go -- go test ./... -v
