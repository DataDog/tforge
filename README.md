# tforge

A minimal, idiomatic Go reimplementation of CDKTF without JSII or Node.js dependencies.

## Overview

tforge enables you to define Terraform infrastructure using pure Go code, generating `.tf.json` files that can be applied with Terraform or OpenTofu.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│  provider   │────▶│   generate   │────▶│   config   │
│  (schemas)  │     │  (Go SDK)    │     │  (stacks)  │
└─────────────┘     └──────────────┘     └─────┬──────┘
                                               │
                                               ▼
                                         ┌──────────┐
                                         │  render  │
                                         │ (.tf.json)│
                                         └──────────┘
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

```go
package main

import (
	"tforge/config"
	"tforge/render"
)

func main() {
	// Create a new stack
	stack := config.NewStack("my-infrastructure")

	// Add a provider
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Version: "~> 5.0",
		Config: map[string]any{
			"region": "us-east-1",
		},
	})

	// Add a resource
	stack.AddResource(config.Resource{
		Type: "aws_instance",
		Name: "web",
		Config: map[string]any{
			"ami":           "ami-0c55b159cbfafe1f0",
			"instance_type": "t2.micro",
		},
	})

	// Render to .tf.json
	if err := render.ToFile(stack, "main.tf.json"); err != nil {
		panic(err)
	}
}
```

## Generating Provider SDKs

```go
package main

import (
	"tforge/generate"
	"tforge/provider"
)

func main() {
	// Fetch the AWS provider schema
	schema, err := provider.FetchSchema("hashicorp/aws", "5.0.0")
	if err != nil {
		panic(err)
	}

	// Generate Go SDK
	gen := generate.New(generate.Config{
		OutputDir:   "./generated/aws",
		PackageName: "aws",
	})

	if err := gen.Generate(schema); err != nil {
		panic(err)
	}
}
```

## Design Philosophy

- **No JSII**: Pure Go, no JavaScript/TypeScript runtime
- **No HCL parsing**: Generates `.tf.json` directly
- **Minimal dependencies**: Standard library where possible
- **Idiomatic Go**: Small, cohesive packages with minimal public APIs
- **Extensible**: Easy to add expressions, imports, diffing later

## Requirements

- Go 1.22+
- Terraform or OpenTofu (for applying generated configurations)
