# 🔨 Terraform, Forged in Go: Introducing tforge

**Define Terraform infrastructure in pure Go: no JSII, no Node.js, no HashiCorp libraries.**

[![CI](https://github.com/DataDog/tforge/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/DataDog/tforge/actions/workflows/ci.yml)
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
go install github.com/DataDog/tforge/cmd/tforge@latest
```

Or build from source:

```bash
git clone https://github.com/DataDog/tforge.git && cd tforge
make build
```

### 2. Generate provider bindings

Create a sub-package for each provider with a `go:generate` directive. The provider package **must be separate from `package main`** to keep generated types in their own namespace:

```
my-project/
├── main.go              # package main — imports ./datadog
└── datadog/
    └── generate.go      # package datadog — go:generate directive
```

```go
// datadog/generate.go
//go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output .
package datadog
```

Then generate the bindings:

```bash
go generate ./...
```

### 3. Define your infrastructure

```go
package main

import (
    "log"

    "github.com/DataDog/tforge/config"
    "github.com/DataDog/tforge/render"
    "your-project/datadog"
)

func main() {
    stack := config.NewStack("monitoring")

    stack.AddProvider(config.Provider{
        Name:    "datadog",
        Source:  "datadog/datadog",
        Version: "~> 3.39",
    })

    // Constructors attach the resource to the stack and return a typed handle.
    // Configure sets attributes via generated functional options; it can be
    // called multiple times to add more attributes later.
    m, err := datadog.NewMonitor(stack, "cpu_high")
    if err != nil {
        log.Fatal(err)
    }
    m.Configure(
        datadog.Monitor_SetName("High CPU Usage"),
        datadog.Monitor_SetType("metric alert"),
        datadog.Monitor_SetQuery("avg(last_5m):avg:system.cpu.user{*} > 90"),
        datadog.Monitor_SetMessage("CPU usage is above 90%"),
    )

    if err := render.ToFile(stack, "main.tf.json", ""); err != nil {
        log.Fatal(err)
    }
}
```

### 4. Apply with Terraform or OpenTofu

```bash
tofu init && tofu plan
```

## Migrating from CDKTF

If you have an existing CDKTF Go project, tforge can automate the scaffolding:

```bash
tforge migrate              # converts cdktf.json to go:generate directives
tforge migrate -dry-run     # preview changes without applying
```

See the full [Migration Guide](docs/migration-guide.md) for step-by-step instructions and before/after examples.

## CLI Usage

```bash
# Generate Go SDK from a provider
tforge generate -provider <namespace/name> -version <version> [flags]

# Fetch and save a provider schema
tforge schema -provider <namespace/name> -version <version> [-output schema.json]

# Migrate a CDKTF project to tforge
tforge migrate [-dir .] [-dry-run]

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
| **Idiomatic** | Small packages, clear APIs, `go generate` friendly, [functional options](https://github.com/uber-go/guide/blob/master/style.md#functional-options) for configuration |

## Requirements

- Go 1.23+
- [OpenTofu](https://opentofu.org/) or [Terraform](https://www.terraform.io/) CLI
- [golangci-lint](https://golangci-lint.run/) for linting:
  ```bash
  brew install golangci-lint  # macOS
  ```

## License

Apache License 2.0: see [LICENSE.md](LICENSE.md) for details.
