# 🔨 Terraform, Forged in Go: Introducing tforge

**Define Terraform infrastructure in pure Go: no JSII, no Node.js, no HashiCorp libraries.**

[![CI](https://github.com/St0rmingBr4in/tforge/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/St0rmingBr4in/tforge/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE.md)

tforge generates typed Go structs from Terraform provider schemas, giving you full flexibility on how you define your infrastructure. It outputs `.tf.json` files that work with both Terraform and OpenTofu.

## Why tforge?

[CDKTF was sunset by IBM](https://developer.hashicorp.com/terraform/cdktf) and while it was powerful, it came with significant complexity: JSII overhead requiring Node.js, multi-language abstraction layers, and tight coupling to HashiCorp's SDK.

tforge takes a different approach:

- **Go-native**: first-class Go support, no other languages
- **No JSII**: pure Go implementation, no Node.js or JavaScript runtime
- **No HashiCorp libraries**: zero `github.com/hashicorp/*` dependencies
- **Minimal**: small codebase with a single external dependency

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│  provider   │────>│   generate   │────>│   config   │
│  (schemas)  │     │   (Go SDK)   │     │  (stacks)  │
└─────────────┘     └──────────────┘     └─────┬──────┘
                                               │
                                               v
                                         ┌────────────┐
                                         │   render   │
                                         │ (.tf.json) │
                                         └────────────┘
```

| Package    | Description                                           |
|------------|-------------------------------------------------------|
| `provider` | Fetch provider binaries and load provider schemas     |
| `generate` | Generate Go SDK code from provider schemas            |
| `config`   | Build Terraform configurations with typed Go structs  |
| `render`   | Serialize configurations to `.tf.json` format         |

## Quick Start

### 1. Install

```bash
go install tforge/cmd/tforge@latest
```

Or build from source:

```bash
git clone https://github.com/St0rmingBr4in/tforge.git && cd tforge
make build
```

### 2. Generate provider bindings

```bash
# Generate Go SDK for the Datadog provider
tforge generate -provider datadog/datadog -version 3.39.0 -output ./generated/datadog -package datadog
```

Or use `go generate` in your code:

```go
//go:generate tforge generate -provider datadog/datadog -version 3.39.0 -output ./generated/datadog -package datadog
```

### 3. Define your infrastructure

```go
package main

import (
    "log"

    "tforge/config"
    "tforge/render"
    "your-project/generated/datadog"
)

func main() {
    stack := config.NewStack("monitoring")

    stack.AddProvider(config.Provider{
        Name:    "datadog",
        Source:  "datadog/datadog",
        Version: "~> 3.39",
    })

    stack.AddResource(config.Resource{
        Type: "datadog_monitor",
        Name: "cpu_high",
        Config: map[string]any{
            "name":    "High CPU Usage",
            "type":    "metric alert",
            "query":   "avg(last_5m):avg:system.cpu.user{*} > 90",
            "message": "CPU usage is above 90%",
        },
    })

    if err := render.ToFile(stack, "main.tf.json"); err != nil {
        log.Fatal(err)
    }
}
```

### 4. Apply with Terraform or OpenTofu

```bash
tofu init && tofu plan
```

## CLI Usage

```bash
# Generate Go SDK from a provider
tforge generate -provider <namespace/name> -version <version> [flags]

# Fetch and save a provider schema
tforge schema -provider <namespace/name> -version <version> [-output schema.json]

# Show version
tforge -v
```

### Generate Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-provider` | Provider source (e.g., `datadog/datadog`) | required |
| `-version` | Provider version | required |
| `-schema` | Load schema from file instead of fetching | - |
| `-output` | Output directory for generated code | `./generated` |
| `-package` | Go package name | `generated` |
| `-comments` | Include doc comments in generated code | `true` |
| `-omitempty` | Add `omitempty` to optional field JSON tags | `true` |

## Development

```bash
# First-time setup (installs pre-commit hook)
make setup

# Run tests
make test

# Run tests with coverage
make coverage

# Lint (requires golangci-lint)
make lint

# Check formatting
make fmt

# Build
make build
```

## Design Principles

| Principle | Description |
|-----------|-------------|
| **Go-native** | Built for Go developers, not a multi-language abstraction |
| **No JSII** | Pure Go, no Node.js, no JavaScript runtime, no interop layers |
| **No HashiCorp deps** | Zero `github.com/hashicorp/*` imports |
| **Minimal** | Standard library where possible, small dependency footprint |
| **OpenTofu-first** | Prefers OpenTofu registry and CLI, falls back to Terraform |
| **Idiomatic** | Small packages, clear APIs, `go generate` friendly |

## Requirements

- Go 1.23+
- [OpenTofu](https://opentofu.org/) or [Terraform](https://www.terraform.io/) CLI
- [golangci-lint](https://golangci-lint.run/) for linting:
  ```bash
  brew install golangci-lint  # macOS
  ```

## License

Apache License 2.0: see [LICENSE.md](LICENSE.md) for details.
