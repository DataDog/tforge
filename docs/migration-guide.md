# Migrating from CDKTF to tforge

This guide walks you through converting a CDKTF Go project to tforge.

## Overview

tforge replaces CDKTF's multi-language abstraction (JSII, Node.js, constructs) with pure Go code generation. The migration removes all CDKTF dependencies and replaces them with idiomatic Go patterns.

| Aspect | CDKTF | tforge |
|--------|-------|--------|
| Runtime | JSII (Node.js + Go interop) | Pure Go |
| Config file | `cdktf.json` | `go:generate` directive |
| Type wrappers | `jsii.String()`, `jsii.Bool()` | Native Go types |
| Provider code | `cdktf get` (npm) | `go generate ./...` |
| Stack output | `app.Synth()` | `render.ToFile()` |
| Dependencies | constructs, jsii-runtime-go, cdktf | tforge only |

## Automated migration

tforge includes a `migrate` command that handles the scaffolding:

```bash
# Preview what will change
tforge migrate -dry-run

# Run the migration
tforge migrate
```

This will:
1. Read `cdktf.json` to extract provider requirements
2. Create a sub-package per provider with a `go:generate` directive
3. Delete `cdktf.json` and `package.json`
4. Update `.gitignore` to ignore `*_gen.go` files

After running `tforge migrate`, you still need to update your Go code manually (see below).

## Step-by-step manual migration

### 1. Project structure

CDKTF generates all provider code into a flat `generated/` directory. tforge uses a **separate Go package per provider**, each containing a `generate.go` file with a `//go:generate` directive.

**Before (CDKTF):**
```
my-project/
├── cdktf.json
├── package.json
├── main.go
└── generated/          # created by `cdktf get`
    └── hashicorp/
        └── aws/
            ├── provider/
            ├── instance/
            └── ...
```

**After (tforge):**
```
my-project/
├── main.go             # package main — imports ./datadog
├── datadog/
│   ├── generate.go     # package datadog — go:generate directive
│   ├── provider_gen.go     # (generated, gitignored)
│   ├── resources_gen.go    # (generated, gitignored)
│   └── data_sources_gen.go # (generated, gitignored)
└── .gitignore          # includes *_gen.go
```

> **Important:** The `generate.go` file must be in a **separate package** named after the provider, not in `package main`. This keeps generated types in their own namespace and avoids polluting the main package. Your `main.go` then imports this package.

### 2. Create the provider package

For each provider in your `cdktf.json`, create a sub-directory with a `generate.go` file:

```go
// datadog/generate.go

// Package datadog provides generated types for the Datadog Terraform provider.
//
//go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output .
package datadog
```

The `-package` flag must match the directory name and the `package` declaration. The `-output .` flag tells tforge to write generated files into the same directory.

Then generate the provider bindings:

```bash
go generate ./...
```

This creates `provider_gen.go`, `resources_gen.go`, and `data_sources_gen.go` in the provider package.

### 3. Update go.mod

Remove CDKTF dependencies and add tforge:

```bash
# Add tforge
go get tforge@latest

# Remove CDKTF dependencies
go get -d github.com/hashicorp/terraform-cdk-go/cdktf@none
go get -d github.com/aws/jsii-runtime-go@none
go get -d github.com/aws/constructs-go/constructs/v10@none

go mod tidy
```

### 4. Update imports

**Remove:**
```go
"github.com/aws/constructs-go/constructs/v10"
"github.com/aws/jsii-runtime-go"
"github.com/hashicorp/terraform-cdk-go/cdktf"
"your-module/generated/hashicorp/aws/instance"
"your-module/generated/hashicorp/aws/provider"
```

**Add:**
```go
"tforge/config"
"tforge/render"
"your-module/datadog"   // your generated provider package
```

### 5. Replace CDKTF patterns

#### Stack creation

```go
// CDKTF
func NewMyStack(scope constructs.Construct, id string) cdktf.TerraformStack {
    stack := cdktf.NewTerraformStack(scope, &id)
    // ...
    return stack
}

func main() {
    app := cdktf.NewApp(nil)
    NewMyStack(app, "my-stack")
    app.Synth()
}
```

```go
// tforge
func NewMyStack() *config.Stack {
    stack := config.NewStack("my-stack")
    // ...
    return stack
}

func main() {
    stack := NewMyStack()
    if err := render.ToFile(stack, "main.tf.json", ""); err != nil {
        log.Fatal(err)
    }
}
```

#### Provider configuration

```go
// CDKTF
provider.NewAwsProvider(stack, jsii.String("aws"), &provider.AwsProviderConfig{
    Region: jsii.String("us-east-1"),
})
```

```go
// tforge
stack.AddProvider(config.Provider{
    Name:    "aws",
    Source:  "hashicorp/aws",
    Version: "~> 5.0",
    Config:  map[string]any{"region": "us-east-1"},
})
```

#### Resource creation

tforge uses a functional options pattern for type-safe resource creation:

```go
// CDKTF
instance.NewInstance(stack, jsii.String("web"), &instance.InstanceConfig{
    Ami:          jsii.String("ami-123"),
    InstanceType: jsii.String("t3.micro"),
    Tags: &map[string]*string{
        "Name": jsii.String("web-server"),
    },
})
```

```go
// tforge — using functional options (type-safe)
aws.NewInstance(stack, "web",
    aws.Instance_SetAmi("ami-123"),
    aws.Instance_SetInstanceType("t3.micro"),
    aws.Instance_SetTags(map[string]string{"Name": "web-server"}),
)
```

```go
// tforge — using struct directly
aws.NewInstance(stack, "web", aws.FromInstance(aws.Instance{
    Ami:          "ami-123",
    InstanceType: "t3.micro",
    Tags:         map[string]string{"Name": "web-server"},
}))
```

```go
// tforge — using the untyped config API (no generated code needed)
stack.AddResource(config.Resource{
    Type: "aws_instance",
    Name: "web",
    Config: map[string]any{
        "ami":           "ami-123",
        "instance_type": "t3.micro",
        "tags":          map[string]string{"Name": "web-server"},
    },
})
```

#### JSII type wrappers

All `jsii.*` wrappers are replaced with native Go values:

| CDKTF | tforge |
|-------|--------|
| `jsii.String("value")` | `"value"` |
| `jsii.Bool(true)` | `true` |
| `jsii.Number(42)` | `42` |
| `jsii.Strings("a", "b")` | `[]string{"a", "b"}` |
| `&map[string]*string{...}` | `map[string]string{...}` |

#### Resource references

```go
// CDKTF — uses method calls on resource objects
container.NewContainer(stack, jsii.String("nginx"), &container.ContainerConfig{
    Image: dockerImage.ImageId(),  // method call returns a reference
})
```

```go
// tforge — uses Terraform interpolation strings
nginxImage := config.Resource{Type: "docker_image", Name: "nginx"}
docker.NewContainer(stack, "nginx",
    docker.Container_SetImage(nginxImage.Ref("image_id")),  // returns "${docker_image.nginx.image_id}"
)
```

#### Data sources

```go
// CDKTF
ami := datasource.NewDataAwsAmi(stack, jsii.String("ubuntu"), &datasource.DataAwsAmiConfig{
    MostRecent: jsii.Bool(true),
})
```

```go
// tforge
aws.NewAMIData(stack, "ubuntu",
    aws.AMIData_SetMostRecent(true),
)
```

### 6. Meta-arguments

tforge supports all Terraform meta-arguments through the `config.Resource` struct:

```go
stack.AddResource(config.Resource{
    Type:      "aws_instance",
    Name:      "web",
    Provider:  "aws.west",                          // provider alias
    DependsOn: []string{"aws_vpc.main"},            // explicit dependencies
    Count:     3,                                    // count
    ForEach:   map[string]string{"a": "1", "b": "2"}, // for_each
    Config:    map[string]any{...},
    Lifecycle: &config.Lifecycle{
        CreateBeforeDestroy: true,
        IgnoreChanges:       []string{"tags"},
    },
    Timeouts: &config.Timeouts{
        Create: "30m",
        Delete: "10m",
    },
})
```

### 7. Variables, outputs, locals, and backends

```go
// Variables
if err := stack.AddVariable(config.Variable{
    Name:        "environment",
    Type:        "string",
    Description: "Deployment environment",
    Default:     "production",
    Validation: &config.VariableValidation{
        Condition:    "length(var.environment) > 0",
        ErrorMessage: "Environment must not be empty",
    },
}); err != nil {
    log.Fatal(err)
}

// Outputs
if err := stack.AddOutput(config.Output{
    Name:      "instance_ip",
    Value:     "${aws_instance.web.public_ip}",
    Sensitive: true,
    DependsOn: []string{"aws_instance.web"},
}); err != nil {
    log.Fatal(err)
}

// Locals
stack.SetLocal("project_name", "my-app")
stack.SetLocal("common_tags", map[string]string{
    "Project":   "my-app",
    "ManagedBy": "tforge",
})

// Backend
stack.SetBackend(config.Backend{
    Type: "s3",
    Config: map[string]any{
        "bucket": "my-terraform-state",
        "key":    "prod/terraform.tfstate",
        "region": "us-east-1",
    },
})
```

### 8. Build and validate

```bash
# Generate provider bindings
go generate ./...

# Build and render the Terraform JSON
go run .

# Initialize and validate with Terraform/OpenTofu
tofu init && tofu validate
```

## Limitations

- **Terraform modules** are not supported. Replace module calls with equivalent resources.
- **Remote state data sources** must use the untyped `stack.AddDataSource(config.DataSource{...})` API.

## Full example: before and after

### Before (CDKTF with Docker provider)

```go
package main

import (
    "github.com/hashicorp/terraform-cdk/examples/go/docker/generated/kreuzwerker/docker/container"
    "github.com/hashicorp/terraform-cdk/examples/go/docker/generated/kreuzwerker/docker/image"
    dockerprovider "github.com/hashicorp/terraform-cdk/examples/go/docker/generated/kreuzwerker/docker/provider"
    "github.com/aws/constructs-go/constructs/v10"
    "github.com/aws/jsii-runtime-go"
    "github.com/hashicorp/terraform-cdk-go/cdktf"
)

func NewMyStack(scope constructs.Construct, id string) cdktf.TerraformStack {
    stack := cdktf.NewTerraformStack(scope, &id)

    dockerprovider.NewDockerProvider(stack, jsii.String("provider"), &dockerprovider.DockerProviderConfig{})

    dockerImage := image.NewImage(stack, jsii.String("nginxImage"), &image.ImageConfig{
        Name:        jsii.String("nginx:latest"),
        KeepLocally: jsii.Bool(false),
    })

    container.NewContainer(stack, jsii.String("nginxContainer"), &container.ContainerConfig{
        Image: dockerImage.ImageId(),
        Name:  jsii.String("tutorial"),
        Ports: &[]*container.ContainerPorts{{
            Internal: jsii.Number(80), External: jsii.Number(8000),
        }},
    })

    return stack
}

func main() {
    app := cdktf.NewApp(nil)
    NewMyStack(app, "my-stack")
    app.Synth()
}
```

### After (tforge)

```
my-project/
├── main.go
└── docker/
    └── generate.go     # //go:generate tforge generate -provider kreuzwerker/docker ...
```

```go
// docker/generate.go
//go:generate tforge generate -provider kreuzwerker/docker -version 4.0.0 -package docker -output .
package docker
```

```go
// main.go
package main

import (
    "log"

    "my-project/docker"

    "tforge/config"
    "tforge/render"
)

func NewMyStack() *config.Stack {
    stack := config.NewStack("my-stack")

    stack.AddProvider(config.Provider{
        Name:    "docker",
        Source:  "kreuzwerker/docker",
        Version: "4.0.0",
    })

    nginxImage := config.Resource{Type: "docker_image", Name: "nginxImage"}
    docker.NewImage(stack, "nginxImage",
        docker.Image_SetName("nginx:latest"),
        docker.Image_SetKeepLocally(false),
    )

    docker.NewContainer(stack, "nginxContainer",
        docker.Container_SetImage(nginxImage.Ref("image_id")),
        docker.Container_SetName("tutorial"),
        docker.Container_SetPorts(docker.NewContainerPorts(
            docker.ContainerPorts_SetInternal(80),
            docker.ContainerPorts_SetExternal(8000),
        )),
    )

    return stack
}

func main() {
    stack := NewMyStack()
    if err := render.ToFile(stack, "main.tf.json", ""); err != nil {
        log.Fatal(err)
    }
}
```
