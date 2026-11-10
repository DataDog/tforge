# tforge Example

This example demonstrates how to use tforge to generate a typed Go SDK for the Datadog Terraform provider and use it to create infrastructure configurations.

## Quick Start

### 1. Generate the Provider SDK

The `datadog/` directory contains a `generate.go` file with a `go:generate` directive:

```go
//go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output .
package datadog
```

Run the generator:

```bash
go generate ./datadog/...
```

This creates:
- `provider_gen.go` - Provider configuration struct
- `resources_gen.go` - All resource structs (76 resources)
- `data_sources_gen.go` - All data source structs (38 data sources)

### 2. Use the Generated Types

```go
package main

import (
    "log"

    "github.com/DataDog/tforge/config"
    "github.com/DataDog/tforge/example/datadog"
    "github.com/DataDog/tforge/render"
)

func main() {
    stack := config.NewStack("my-infrastructure")

    // Configure provider
    stack.AddProvider(config.Provider{
        Name:    "datadog",
        Source:  "datadog/datadog",
        Version: "~> 3.39",
        Config: map[string]any{
            "api_url": "https://api.datadoghq.eu/",
        },
    })

    // Create a monitor: the constructor attaches it to the stack and
    // returns a typed handle. Configure sets attributes via generated
    // functional options and can be called multiple times.
    m, err := datadog.NewMonitor(stack, "cpu_alert")
    if err != nil {
        log.Fatal(err)
    }
    m.Configure(
        datadog.Monitor_SetName("High CPU Alert"),
        datadog.Monitor_SetType("metric alert"),
        datadog.Monitor_SetMessage("CPU usage is high!"),
        datadog.Monitor_SetQuery("avg(last_5m):avg:system.cpu.user{*} > 80"),
        datadog.Monitor_SetMonitorThresholds(&datadog.MonitorMonitorThresholds{
            Critical: "80",
            Warning:  "60",
        }),
    )

    // Render to .tf.json
    if err := render.ToFile(stack, "main.tf.json", ""); err != nil {
        log.Fatal(err)
    }
}
```

### 3. Run the Example

```bash
go run main.go
```

This outputs a valid Terraform JSON configuration.

### 4. Apply with Terraform

```bash
export DD_API_KEY="your-api-key"
export DD_APP_KEY="your-app-key"
terraform init
terraform plan
terraform apply
```

## Project Structure

```
example/
├── README.md           # This file
├── main.go             # Example usage
└── datadog/
    ├── generate.go         # go:generate directive
    ├── provider_gen.go     # Generated provider config
    ├── resources_gen.go    # Generated resource types
    └── data_sources_gen.go # Generated data source types
```

## Adding Another Provider

To generate types for a different provider (e.g., AWS):

1. Create a new directory:
   ```bash
   mkdir aws
   ```

2. Add a `generate.go` with the generate directive:
   ```go
   //go:generate tforge generate -provider hashicorp/aws -version 5.0.0 -package aws -output .
   package aws
   ```

3. Run generation:
   ```bash
   go generate ./aws/...
   ```

## CLI Options

```bash
tforge generate [options]

Options:
  -provider string    Provider source (e.g., datadog/datadog)
  -version string     Provider version (e.g., 3.39.0)
  -schema string      Path to cached schema JSON (alternative to -provider/-version)
  -output string      Output directory (default: ./generated)
  -package string     Go package name (default: provider name)
  -comments           Generate documentation comments (default: true)
  -omitempty          Add omitempty to optional fields (default: true)
```

## Tips

- **Caching schemas**: Use `tforge schema -provider X -version Y -output schema.json` to save a schema locally, then generate with `-schema schema.json` for faster rebuilds.

- **CI/CD**: Add `go generate ./...` to your CI pipeline to ensure generated code is up-to-date.

- **Version pinning**: Always specify exact provider versions in `go:generate` directives to ensure reproducible builds.
