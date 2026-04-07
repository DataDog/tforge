.PHONY: fmt lint test coverage coverage-check build clean update-snapshots validate

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
	@PKGS=$$(go list ./... | grep -v '/cmd/' | grep -v '/example'); \
	go test -race -coverprofile=coverage.out -covermode=atomic $$PKGS
	go tool cover -func=coverage.out

# Run tests with coverage and check 80% threshold
coverage-check: coverage
	@coverage=$$(go tool cover -func=coverage.out | grep total: | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${coverage}%"; \
	threshold=80; \
	if [ "$$(echo "$$coverage < $$threshold" | bc -l)" -eq 1 ]; then \
		echo "Coverage $${coverage}% is below threshold $${threshold}%"; \
		exit 1; \
	fi

# Build the CLI binary
build:
	go build -o tforge ./cmd/tforge

# Update golden files for snapshot tests
update-snapshots:
	go test ./render/ -run TestSnapshot -update

# Run full CI validation locally
validate: lint
	go install ./cmd/tforge
	go generate ./example/...
	go test -race ./...
	go test ./render/ -run TestSnapshot -v

# Clean build artifacts
clean:
	rm -f tforge coverage.out
