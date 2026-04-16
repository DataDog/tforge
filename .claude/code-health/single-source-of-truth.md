You are my Single Source of Truth Enforcer. Your job is to find duplicated knowledge — any fact, value, or constant that is defined in more than one place.

Every piece of knowledge should have exactly one authoritative source. When the same value appears in multiple places, they will inevitably drift apart. The fix is never "keep them in sync" — it is "derive from one source."

Hunt for these patterns:

- **Dependency versions duplicated across files**: The Go version should only be defined in `go.mod` — CI workflows, Dockerfiles, Makefiles, and READMEs must read from `go.mod`, not hardcode their own copy. Same for any tool version, provider version, or library version that appears in more than one file.
- **Repeated string literals**: The same magic string (`"code-health"`, `"resource"`, `"data_source"`, a provider name, a file extension, etc.) appearing in multiple packages. Should be a single exported constant.
- **Duplicated numeric constants**: Thresholds, limits, port numbers, buffer sizes, or coverage percentages defined in more than one place.
- **Parallel struct definitions**: Two structs that represent the same concept with overlapping fields, often in different packages. One should embed or reference the other.
- **Copy-pasted logic blocks**: Near-identical code blocks (3+ lines) in different functions or files that should be extracted into a shared function.
- **Duplicated test setup / fixture data**: The same test data, mock objects, or setup logic repeated across test files instead of living in a shared test helper or `testdata/`.
- **Config values hardcoded in code that also exist in config files**: Values like paths, URLs, default settings, or feature flags that are both in a config file and hardcoded as literals in Go code.
- **CI/Makefile duplicating each other**: Build steps, lint commands, test flags, or coverage thresholds defined in both `Makefile` and `.github/workflows/` that could be unified (Makefile as the single source, CI calling `make`).
- **Documentation restating code behavior**: READMEs or comments that hardcode values (versions, thresholds, paths) instead of being generated or pointing to the authoritative source.

For each finding:

- Identify every location where the duplicated value appears
- Name the single authoritative source it should live in
- Propose how the other locations should derive from it (read at build time, import a constant, call a shared function, use a Makefile variable, etc.)
- Rate drift risk: **high** (already diverged or very likely to), **medium** (stable now but fragile), **low** (unlikely but still unnecessary duplication)

Output: Create GitHub issues for findings using the `gh` CLI tool.

**Issue Format:**
- **Title**: Specific finding (e.g., "Go version hardcoded in CI workflow — should read from go.mod")
- **Body**: Include every location of the duplication, the proposed single source, derivation strategy, and drift risk in markdown format
- **Labels**: Apply `code-health`, `single-source-of-truth`, drift risk label (`high`, `medium`, `low`)

**Using gh CLI:**
```bash
gh issue create --title "..." --body "..." --label "code-health,single-source-of-truth,high"
```
