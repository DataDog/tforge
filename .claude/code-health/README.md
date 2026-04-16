# Code Health Agents

This directory contains Claude Code agent configurations for automated code health maintenance. Each file defines a specialized agent that can be run to analyze the codebase and create or fix GitHub issues.

## Agents

| File | Role | What it does |
|------|------|--------------|
| `cleanup.md` | Repo Janitor | Finds committed generated files, stale docs, debug scaffolding, and other clutter. Creates GitHub issues for cleanup tasks. |
| `test-coverage.md` | Test Strategist | Identifies high-risk test gaps based on critical paths and edge cases. Creates GitHub issues with specific test cases. |
| `yagni-simplifier.md` | Simplicity Enforcer | Finds over-engineered abstractions, unnecessary configuration, and excess indirection. Creates GitHub issues ranked by simplicity gain. |
| `fail-loud.md` | Fail-Loud Auditor | Finds silently ignored errors, swallowed panics, fallbacks masking failures, and warnings that should be fatal. Creates GitHub issues ranked by severity. |
| `go-strictness.md` | Go Strictness Enforcer | Finds weak typing: `any`/`interface{}`, raw strings as enums, `map[string]any` data bags, unchecked type assertions, stringly-typed errors, and other type-system bypasses. Creates GitHub issues ranked by effort. |
| `single-source-of-truth.md` | Single Source of Truth Enforcer | Finds duplicated knowledge: dependency versions in multiple files, repeated string/numeric constants, copy-pasted logic, parallel structs, and CI/Makefile overlap. Creates GitHub issues ranked by drift risk. |
| `swarm-fix.md` | Code Health Fixer | Picks the top 1-3 open code-health issues and implements fixes end-to-end, one commit per issue. |

## Usage

These agents are invoked via Claude Code. Each agent reads from the codebase and interacts with GitHub issues using the `gh` CLI.

The typical workflow is:

1. Run an analysis agent (`cleanup.md`, `test-coverage.md`, or `yagni-simplifier.md`) to create issues
2. Run the fixer agent (`swarm-fix.md`) to implement the highest-ROI fixes

All created issues use the `code-health` label for tracking.
