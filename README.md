# tforge

A Go-native replacement for [CDKTF](https://developer.hashicorp.com/terraform/cdktf) — define TF infrastructure in pure Go.

## Why tforge?

CDKTF was sunset by IBM and while it was powerful it came with significant complexity:
- **JSII overhead**: Requires Node.js runtime and JavaScript interop layer
- **Multi-language complexity**: Supports many languages, adding abstraction layers
- **HashiCorp dependencies**: Tightly coupled to HashiCorp's SDK and tooling

**tforge** takes a different approach:
- **Go only**: First-class Go support, no other languages
- **No JSII**: Pure Go implementation, no Node.js or JavaScript runtime
- **No HashiCorp libraries**: Zero dependencies on `github.com/hashicorp/*`
- **Minimal**: Small codebase, easy to understand and extend

## Overview

tforge generates typed Go structs from Terraform provider schemas, letting you define infrastructure with full IDE support and compile-time type checking. It outputs `.tf.json` files that work with both Terraform and OpenTofu.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│  provider   │────▶│   generate   │────▶│   config   │
│  (schemas)  │     │   (Go SDK)   │     │  (stacks)  │
└─────────────┘     └──────────────┘     └─────┬──────┘
                                               │
                                               ▼
                                         ┌────────────┐
                                         │  render    │
                                         │ (.tf.json) │
                                         └────────────┘
```

## Packages

| Package    | Description                                           |
|------------|-------------------------------------------------------|
| `provider` | Fetch provider binaries and load provider schemas     |
| `schema`   | Normalize and navigate provider schema structures     |
| `generate` | Generate Go SDK code from provider schemas            |
| `config`   | Build Terraform configurations with typed Go structs  |
| `render`   | Serialize configurations to `.tf.json` format         |

## Quick Start

For a runnable demo, see [`example/main.go`](./example/main.go).

#### Steps

1. Generate SDK code for your provider(s):
   ```
   go generate ./...
   ```
2. Implement your stack as shown above, using the generated types for resources and providers.
3. Render your stack to `.tf.json` for use with Terraform or OpenTofu:
   ```
   go run main.go > main.tf.json
   ```



## Design Principles

| Principle | Description |
|-----------|-------------|
| **Go-native** | Built for Go developers, not a multi-language abstraction |
| **No JSII** | Pure Go, no Node.js, no JavaScript runtime, no interop layers |
| **No HashiCorp deps** | Zero `github.com/hashicorp/*` imports |
| **Minimal** | Standard library where possible, small dependency footprint |
| **OpenTofu-first** | Prefers OpenTofu, falls back to Terraform CLI |
| **Idiomatic** | Small packages, clear APIs, `go generate` friendly |

## Requirements

- Go 1.22+
- Terraform or OpenTofu

## License

Apache License 2.0 - see [LICENSE.md](LICENSE.md) for details.
