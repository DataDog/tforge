.PHONY: fmt lint shellcheck test coverage coverage-check build install clean setup update-snapshots pre-commit snapshot-tests validate-golden clear-golden-cache migration-test

# Terraform/OpenTofu versions to validate golden files against (requires Docker)
TERRAFORM_VERSIONS = 1.5 1.9 latest
TOFU_VERSIONS = 1.8 latest

# Packages excluded from lint and coverage (generated code, CLI glue)
EXCLUDED = /cmd/ /example /testdata
PKGS = $(shell go list ./... $(foreach e,$(EXCLUDED),| grep -v '$(e)'))
# Lint needs relative paths, not module-prefixed
LINT_PKGS = $(shell go list ./... $(foreach e,$(EXCLUDED),| grep -v '$(e)') | sed "s|^$$(go list -m)/|./|")

# Format check (same as CI)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Lint (requires golangci-lint installed locally)
lint: fmt shellcheck
	golangci-lint run $(LINT_PKGS)

# Lint shell scripts (requires shellcheck)
shellcheck:
	shellcheck scripts/*.sh

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

# Install the CLI binary to GOPATH/bin
install:
	go install ./cmd/tforge

# Regenerate golden files
update-snapshots: install
	go generate ./testdata/golden/...
	@for dir in testdata/golden/*/; do \
		go run ./$$dir > $${dir}main.tf.golden.json; \
	done

# Verify golden files match generated output (runs with both CLIs)
snapshot-tests: install
	@for cli in terraform tofu; do \
		echo "=== Snapshot tests with $$cli ==="; \
		TFORGE_CLI=$$cli go generate ./testdata/golden/...; \
		for dir in testdata/golden/*/; do \
			echo "Testing $$dir..."; \
			TFORGE_CLI=$$cli go run ./$$dir | diff - $${dir}main.tf.golden.json || exit 1; \
		done; \
	done

# Validate golden files against multiple Terraform and OpenTofu versions (requires Docker)
validate-golden:
	TERRAFORM_VERSIONS="$(TERRAFORM_VERSIONS)" TOFU_VERSIONS="$(TOFU_VERSIONS)" ./scripts/validate-golden.sh

clear-golden-cache:
	./scripts/validate-golden.sh --clear-cache

# Run migration tests with both CLIs
migration-test: install
	@for cli in terraform tofu; do \
		echo "=== Migration tests with $$cli ==="; \
		TFORGE_CLI=$$cli ./scripts/migration-test.sh; \
	done

# Run full CI validation locally (pre-commit).
# Keep these dependencies in sync with CI jobs in .github/workflows/ci.yml.
pre-commit: lint coverage-check snapshot-tests validate-golden migration-test

# Clean build artifacts
# One-time setup after cloning (installs pre-commit hook)
setup:
	ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit
	@echo "Pre-commit hook installed (.git/hooks/pre-commit -> .githooks/pre-commit)"

# Clean build artifacts
clean:
	rm -f tforge coverage.out
