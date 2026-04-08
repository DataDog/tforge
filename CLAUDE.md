# tforge - Terraform Infrastructure in Pure Go

Go library that generates typed structs from Terraform provider schemas. No JSII, no Node.js, no HashiCorp libraries. Outputs `.tf.json` files compatible with Terraform and OpenTofu.

## Project Structure

```
tforge/
├── cmd/tforge/         # CLI entry point
├── config/             # Stack, Provider, Resource, DataSource, Variable, Output, Local types
├── generate/           # Code generation from provider schemas -> Go SDK
├── provider/           # Fetch provider binaries and load schemas
├── render/             # Serialize configurations to .tf.json
├── internal/           # Internal utilities
├── example/            # Example usage with Datadog provider (uses go:generate)
│   └── datadog/        # Generated types (gitignored, created by go generate)
├── testdata/           # Test fixtures
├── Makefile            # Build, test, lint, coverage targets
└── .golangci.yml       # Linter configuration
```

## Key Workflow: go generate

Generated files (`*_gen.go`) are gitignored. The example uses `go:generate` to create provider bindings at build time:

```bash
go generate ./example/...   # generates example/datadog/resources_gen.go
```

Run `go generate` before testing examples locally.

## Development

```bash
make test               # run tests with race detector
make lint               # golangci-lint (excludes example/ and *_gen.go)
make coverage           # tests with coverage report
make build              # build CLI binary
make update-snapshots   # update golden files for snapshot tests
make fmt                # check formatting
```

## Code Style

- Go 1.22+, standard library preferred
- Single external dependency (minimal footprint)
- `gofmt` for formatting, `golangci-lint` for linting
- Check error returns (errcheck is enabled)
- Generated code is excluded from linting (`.golangci.yml`)

## Testing

- Unit tests alongside source files
- Snapshot tests in `render/` with golden files (`render/testdata/*.golden.json`)
- Update snapshots: `go test ./render/ -run TestSnapshot -update`
- Example validation: `go test ./example/ -v` (requires `go generate` first)
- CI enforces 80% coverage threshold (excludes `cmd/` and `example/`)

## CI

GitHub Actions (`.github/workflows/ci.yml`):
- **Lint**: gofmt check + golangci-lint
- **Test**: race detector + coverage with 80% threshold
- **Snapshot Tests**: golden file comparison

## Review Guidelines

- Keep PRs small and focused
- Check error returns (errcheck linter will catch these)
- Do not commit generated files (`*_gen.go`, `*.tf.json`)
- Add tests for new functionality
- Run `make lint` before submitting
