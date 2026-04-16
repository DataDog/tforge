# Changelog

All notable changes to tforge are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-04-10

### Added

- **Terraform module support** (`config.Module`, `Stack.AddModule()`)
  - Declare module calls with source, version, input variables, and provider mappings
  - Meta-arguments: `depends_on`, `count`, `for_each`
  - Renders `module {}` blocks in `.tf.json` output
  - Reference module outputs via `Module.Ref("output_name")`

## [0.1.0] - 2026-04-10

Initial release of tforge.

### Added

- **Code generation** from Terraform provider schemas to typed Go structs
  - Functional options API (`Resource_SetField(value)`) for type-safe resource configuration
  - Struct-based API (`FromResource(Resource{...})`) for bulk configuration
  - Support for all attribute types: string, number, bool, list, set, map, object, nested types
  - Nested block support with all nesting modes (single, list, set, map)
  - Generated doc comments from provider schema descriptions
  - Name collision avoidance for resources sharing type names with nested blocks
- **Stack configuration** API (`config` package)
  - Resources, data sources, providers, variables, outputs, locals, backends
  - All meta-arguments: `depends_on`, `count`, `for_each`, `provider`, `lifecycle`, `timeouts`
  - Provisioners and connection blocks
  - Duplicate resource/data source detection
- **JSON rendering** to `.tf.json` format (`render` package)
  - Compatible with both Terraform and OpenTofu
  - Deterministic output with metadata comments
- **Provider schema fetching** (`provider` package)
  - Automatic provider binary download via OpenTofu/Terraform CLI
  - Provider plugin caching (`TF_PLUGIN_CACHE_DIR`)
  - `TFORGE_CLI` env var to force `tofu` or `terraform`
  - Schema loading from JSON files for offline/cached usage
- **CLI** (`tforge` command)
  - `tforge generate` — generate Go SDK from a provider schema
  - `tforge schema` — fetch and save a provider schema as JSON
  - `tforge migrate` — migrate CDKTF Go projects to tforge
  - `tforge version` — print version information
- **CDKTF migration** (`tforge migrate`)
  - Parses `cdktf.json` provider requirements (string and object formats)
  - Creates per-provider sub-packages with `go:generate` directives
  - Removes CDKTF files (`cdktf.json`, `package.json`)
  - Updates `.gitignore` for generated files
  - Dry-run mode for previewing changes
  - Warns about unsupported Terraform modules
- **Testing infrastructure**
  - Snapshot tests with golden files validated against multiple Terraform/OpenTofu versions
  - Migration tests comparing CDKTF and tforge output
  - Docker-based cross-version validation (Terraform 1.5, 1.9, latest; OpenTofu 1.8, latest)
  - 91%+ code coverage with CI enforcement (80% threshold)
- **Documentation**
  - Migration guide (`docs/migration-guide.md`) with before/after examples
  - Contributing guide (`CONTRIBUTING.md`) with versioning policy and release process
  - GitHub Actions CI and release workflows
- **Go canonical initialisms** for generated identifiers (API, URL, ID, HTTP, etc.)
  - Configurable via `Config.Initialisms`

### Design decisions

- **No JSII**: pure Go, no Node.js or JavaScript runtime
- **No HashiCorp libraries**: zero `github.com/hashicorp/*` dependencies
- **Single external dependency**: [jennifer](https://github.com/dave/jennifer) for Go code generation
- **OpenTofu-first**: prefers OpenTofu registry and CLI, falls back to Terraform
