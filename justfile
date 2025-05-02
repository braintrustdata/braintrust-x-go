
# Verify the build for ci.
ci: clean fmt lint vet test build

build:
    go build .

run:
    go run .

clean:
    go clean
    rm -f coverage.out coverage.html

test:
    go test ./... -v

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

# Format, test and run the application
dev: fmt test run
