# Contributing to tforge

## Development setup

```bash
git clone https://github.com/St0rmingBr4in/tforge.git && cd tforge
make setup    # installs pre-commit hook
make test     # run tests
make lint     # run linter
```

See the [README](README.md) for full development commands.

## Versioning

tforge follows [Semantic Versioning](https://semver.org/) (SemVer):

- **Major** (`v1.0.0` -> `v2.0.0`): Breaking changes to the public Go API (`config`, `render`, `generate`, `provider` packages) or CLI flags.
- **Minor** (`v0.1.0` -> `v0.2.0`): New features, new provider support, new CLI commands. Backward-compatible.
- **Patch** (`v0.1.0` -> `v0.1.1`): Bug fixes, documentation updates, test improvements. Backward-compatible.

The version is defined in `cmd/tforge/main.go` as the `version` constant and passed explicitly to `render.ToJSON` / `render.ToFile` as a parameter.

While tforge is pre-1.0, minor versions may include breaking changes. The API will stabilize at v1.0.0.

## Release process

### 1. Update the version

Edit the version constant in `cmd/tforge/main.go`:

```go
const version = "0.2.0"
```

### 2. Commit and tag

```bash
git add cmd/tforge/main.go
git commit -m "Release v0.2.0"
git tag v0.2.0
git push origin main --tags
```

### 3. GitHub Release

Pushing a `v*` tag triggers the [release workflow](.github/workflows/release.yml), which automatically:
- Runs the full CI test suite
- Creates a GitHub Release with auto-generated release notes
- Builds binaries for Linux (amd64, arm64) and macOS (amd64, arm64)
- Attaches the binaries to the release

To create a release manually instead:

```bash
gh release create v0.2.0 --generate-notes
```

### 4. Verify

After the release is published, verify installation works:

```bash
go install tforge/cmd/tforge@v0.2.0
tforge -v   # should print "tforge version 0.2.0"
```

## Pull requests

- Keep PRs small and focused
- Run `make lint` before submitting
- Add tests for new functionality
- Do not commit generated files (`*_gen.go`, `*.tf.json`)
- Check error returns (errcheck linter will catch these)
- Delete feature branches after merging (GitHub auto-delete is enabled; for manual cleanup run `git fetch --prune`)

## Code style

- Go 1.23+, standard library preferred
- `gofmt` for formatting, `golangci-lint` for linting
- Generated code is excluded from linting (`.golangci.yml`)
