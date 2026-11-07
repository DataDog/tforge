# Changelog

All notable changes to tforge are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.4.0] - 2026-05-27

Pre-1.0 minor with breaking API changes. See **Breaking changes** below before upgrading.

### Breaking changes

- **Render API: `version` parameter removed.** `render.ToJSON(stack)` and `render.ToFile(stack, path)` no longer take a version string — they read it from `tforge.Version`, overridable at link time via `-X github.com/DataDog/tforge.Version=...`. Update call sites by dropping the trailing argument.
- **Resource/data-source constructors no longer take a `resourceOpts` parameter.** Generated `NewFoo(stack, name, ...)` returns a typed handle; configure attributes and meta-arguments via `handle.Configure(...)` and `handle.ConfigureResourceOptions(...)` (renamed from the short-lived `Apply` / `ApplyResourceOptions`).
- **`Stack.AddResource` / `Stack.AddDataSource` take interfaces.** Now accept `Resourcer` / `DataSourcer` (pointer-only). Passing a `Resource{}` or `DataSource{}` value literal no longer compiles — pass `&Resource{...}` or a generated typed handle. `Stack.AddResourcePtr` is removed (collapsed into `AddResource`). `Stack.Resources` / `Stack.DataSources` are now `[]Resourcer` / `[]DataSourcer`.
- **`Config` field renamed.** `Resource.Config` and `DataSource.Config` → `Attributes`. `Module.Config` → `Variables`. `Config` is no longer a field of `MetaArguments` — it lives directly on `Resource`, `DataSource`, and `Module`. `MetaArguments` retains only `DependsOn`, `Count`, and `ForEach`.
- **`MaxItems = 1` nested blocks now collapse to a single object.** Generated structs and setters use `*T` instead of `[]*T` (matching cdktf). Affects every provider; regenerate downstream packages.
- **Caller-provided `Name` is preserved.** `AddResource` / `AddDataSource` / `AddModule` no longer mutate `Name`. Use the new `SanitizedName()` method (or `Address()` / `Ref()`, which route through it) to get the Terraform-identifier-valid form.
- **CLI flag rename.** `tforge schema -provider-binary` → `-provider-zip-file`. The flag now accepts only release `.zip` archives and parses the version from the archive filename (the binary-name parse was unreliable).

### Added

- **`Stack.RequiredVersion`** for `terraform.required_version`, e.g. `stack.RequiredVersion = ">= 1.3.7"`.
- **Generated provider metadata + `NewProvider` helper.** Every generated package now emits `var Provider = config.ProviderMetadata{Name, Source, Version}` plus `NewProvider(stack, opts...)`, so consumers stop repeating `Source` / `Version` literals at call sites.
- **Hostname-qualified provider sources.** `ParseSource` now accepts the three-segment form (`registry.opentofu.org/hashicorp/aws`) and round-trips it through `required_providers`.
- **`tforge schema -provider-zip-file` / `tforge generate -provider-zip-file`.** Extract schemas hermetically from a local provider release archive via `dev_overrides`, with no network access.
- **`ResourceOption` functional options** in `config`: `WithLifecycle`, `WithProvider`, `WithDependsOn`, `WithTimeouts`. Applied via `handle.ConfigureResourceOptions(...)`.
- **`NewTerraformHclModule`** constructor and `TerraformHclModuleConfig` (cdktf-compatible convenience API), with a `Count` field on `TerraformHclModuleConfig`.
- **`config.SanitizeName`** for resource / data-source / module names. Forward slashes become `--`; other non-identifier characters are stripped. Idempotent. Called implicitly on insertion.
- **cdktf-compatible `//` metadata block** emitted per resource, data source, and module (`{"//": {"metadata": {"path": "<stack>/<name>", "uniqueId": "<name>"}}}`). Informational — Terraform ignores `//` keys; downstream cdktf-aware tooling keeps working.
- **Single `tforge.Version` variable** at the module root, overridable at link time. The release workflow now injects the tag (`-X github.com/DataDog/tforge.Version=${GITHUB_REF_NAME}`).

### Changed

- **Local provider name** in generated packages now derives from the schema's resource prefix (e.g. `google_*` → `google`) instead of the source's last `/`-segment. Fork and mirror sources like `mycorp/googlebeta` now produce correct bindings.
- **Provider schema-prefix check** demoted from a hard rejection to a `<prefix>-<variant>` allowance, so `hashicorp/google-beta` schemas paired with `google_*` resources are accepted.
- **`config.Provider` shape** consolidated into a single `ProviderMetadata` struct in `config/`. Generated packages expose `Provider.Name` / `Provider.Source` / `Provider.Version` instead of three top-level constants.

### Fixed

- **Plugin-cache symlink dereferencing** in `ensureInitCaches` (`cp -aL`). Fixes spurious `there is no package for ... cached in .terraform/providers` after Docker Desktop restarts on macOS.
- **`ConfigureResourceOptions` on a zero-value handle** no longer panics; it no-ops instead of nil-derefing.
- **`resolveAttributes`** now flags an explicit but empty `Attributes` map as a conflict with `TypedAttributes` (previously silently passed).

### Removed

- `ResourcePrefix` helper (unused after the source-prefix check was relaxed).
- `Stack.AddResourcePtr` (folded into `AddResource(Resourcer)`).

### Internal / testing / CI

- All tests converted to `stretchr/testify/require` (net −1183 lines).
- Round-trip generator test lifted from an embedded source string to a real `internal/handletest` fixture package, gated on `//go:build integration` and run via `make handletest`.
- CI triggers on every PR regardless of base branch (Graphite stack friendly).
- `version-check` job demoted to advisory warning so stacked PRs don't all need to bump.
- `tforge.Version` wired through release builds and CI so binaries no longer report `dev`.
- `version-check` CI job now hard-fails when the VERSION file is bumped without a matching `## [<new>]` section in `CHANGELOG.md`.

## [0.3.0] - 2026-04-27

### Breaking changes

- **`Config` moved out of `MetaArguments`.** It is now a direct field on `Resource`, `DataSource`, and `Module`. `MetaArguments` retains only `DependsOn`, `Count`, and `ForEach`. Module variables, previously accessed via `module.MetaArguments.Config`, are accessed via `module.Config` (and renamed to `module.Variables` later in the same release — see below).
- **`Config` renamed.** `Resource.Config` and `DataSource.Config` → `Attributes`; `Module.Config` → `Variables`.

### Added

- **Generated-code golden files.** Every `*_gen.go` now has a `*.go.golden` companion in `testdata/golden/`, asserted by `TestSnapshots`. Catches generator regressions (silently dropped files, output drift).
- **CI versioning automation.** `version-check` PR job (warns when `VERSION` isn't bumped), `auto-tag` main-branch job (creates `v<VERSION>` annotated tags once CI passes), and a `notify-provider-repos` job in `release.yml` that dispatches to `tforge-provider-datadog` on release publication.
- **CODEOWNERS** assigning all files to `@DataDog/api-reliability`.

### Fixed

- Snapshot test golden file comparison now fails on missing goldens instead of silently skipping.
- `ensureInitCaches` checks errors from `docker volume create` and `os.MkdirAll`.

### Internal

- Extracted `pluginCacheDir` to a package-level constant.
- `goldenExcludes` replaced with `isGoldenExcluded` helper that derives child exclusions from parents via `strings.HasPrefix`.
- Snapshot tests flatten `generated_code` and `json_output` into sibling subtests so a fatal in one no longer skips the other.

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
