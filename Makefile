.PHONY: fmt lint test coverage coverage-check build clean update-snapshots validate snapshot-tests

# Packages excluded from lint and coverage (generated code, CLI glue)
EXCLUDED = /cmd/ /example /testdata
PKGS = $(shell go list ./... $(foreach e,$(EXCLUDED),| grep -v '$(e)'))
# Lint needs relative paths, not module-prefixed
LINT_PKGS = $(shell go list ./... $(foreach e,$(EXCLUDED),| grep -v '$(e)') | sed "s|^$$(go list -m)/|./|")

# Format check (same as CI)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Lint (requires golangci-lint installed locally)
lint: fmt
	golangci-lint run $(LINT_PKGS)

# Run tests with race detector
test:
	go test -race ./...

# Run tests with coverage report
coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic $(PKGS)
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

# Regenerate golden files
update-snapshots:
	go install ./cmd/tforge
	go generate ./testdata/golden/...
	@for dir in testdata/golden/*/; do \
		go run ./$$dir > $${dir}main.tf.golden.json; \
	done

# Verify golden files match generated output
snapshot-tests:
	go install ./cmd/tforge
	go generate ./testdata/golden/...
	@for dir in testdata/golden/*/; do \
		echo "Testing $$dir..."; \
		go run ./$$dir | diff - $${dir}main.tf.golden.json || exit 1; \
	done

# Run full CI validation locally (pre-commit).
# Keep these dependencies in sync with CI jobs in .github/workflows/ci.yml.
validate: lint coverage-check snapshot-tests

# Clean build artifacts
clean:
	rm -f tforge coverage.out
