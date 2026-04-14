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

**Important:** The `generate.go` file for a given provider must always be in a **separate package** named after the provider, not in `package main`. This keeps generated types in their own namespace and avoids polluting the main package:

```
example/
├── main.go                 # package main — imports example/datadog
└── datadog/
    └── generate.go         # package datadog — //go:generate tforge generate -package datadog -output .

testdata/golden/datadog/
├── main.go                 # package main — imports testdata/golden/datadog/datadog
└── datadog/
    └── generate.go         # package datadog

testdata/golden/name_collision/
├── main.go                 # package main — imports testdata/golden/name_collision/aws
└── aws/
    └── generate.go         # package aws
```

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

- Go 1.23+, standard library preferred
- Single external dependency (minimal footprint)
- `gofmt` for formatting, `golangci-lint` for linting
- Check error returns (errcheck is enabled)
- Generated code is excluded from linting (`.golangci.yml`)

## Testing

- Unit tests alongside source files
- Integration tests use `//go:build integration` tag (snapshot, migration, golden validation)
- Snapshot tests with golden files (`testdata/golden/*/main.tf.golden.json`)
- Update snapshots: `make update-snapshots`
- Example validation: `go test ./example/ -v` (requires `go generate` first)
- CI enforces a coverage threshold (see `COVERAGE_THRESHOLD` in `Makefile`; excludes `cmd/` and `example/`)

## CI

GitHub Actions (`.github/workflows/ci.yml`):
- **Lint**: gofmt check + golangci-lint
- **Test**: race detector + coverage with threshold (see `COVERAGE_THRESHOLD` in `Makefile`)
- **Snapshot Tests**: golden file comparison
- **Migration Tests**: CDKTF migration correctness

## Review Guidelines

- Keep PRs small and focused
- Check error returns (errcheck linter will catch these)
- Do not commit generated files (`*_gen.go`, `*.tf.json`)
- Add tests for new functionality
- Run `make lint` before submitting
