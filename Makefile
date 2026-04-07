.PHONY: fmt lint test coverage build clean

# Format check (same as CI)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Lint (requires golangci-lint installed locally)
lint: fmt
	golangci-lint run $$(go list ./... | grep -v '/example' | sed "s|^$$(go list -m)/|./|")

# Run tests with race detector
test:
	go test -race ./...

# Run tests with coverage report
coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

# Build the CLI binary
build:
	go build -o tforge ./cmd/tforge

# Clean build artifacts
clean:
	rm -f tforge coverage.out
