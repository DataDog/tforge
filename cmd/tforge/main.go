// Command tforge is a CLI tool for generating Go SDK code from Terraform provider schemas.
//
// Usage with go:generate:
//
//	//go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output ./datadog
//
// Or from the command line:
//
//	tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output ./generated/datadog
//	tforge schema -provider datadog/datadog -version 3.39.0 -output schema.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"tforge/generate"
	"tforge/provider"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate", "gen":
		generateCmd(os.Args[2:])
	case "schema":
		schemaCmd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("tforge version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tforge - Generate Go SDK from Terraform provider schemas

Usage:
  tforge <command> [options]

Commands:
  generate    Generate Go SDK code from a provider schema
  schema      Fetch and save a provider schema as JSON
  version     Print version information
  help        Show this help message

Examples:
  # Generate Go code for a provider (go:generate compatible)
  //go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog

  # Fetch and save provider schema
  tforge schema -provider hashicorp/aws -version 5.0.0 -output aws-schema.json

  # Generate from a cached schema file
  tforge generate -schema ./schema.json -package mypackage -output ./generated`)
}

func generateCmd(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	providerSource := fs.String("provider", "", "Provider source (namespace/name), e.g., datadog/datadog")
	providerVersion := fs.String("version", "", "Provider version, e.g., 3.39.0")
	schemaFile := fs.String("schema", "", "Path to cached schema JSON file (alternative to -provider/-version)")
	outputDir := fs.String("output", "./generated", "Output directory for generated code")
	packageName := fs.String("package", "", "Go package name (defaults to provider name)")
	comments := fs.Bool("comments", true, "Generate documentation comments")
	omitEmpty := fs.Bool("omitempty", true, "Add omitempty to optional JSON tags")

	fs.Usage = func() {
		fmt.Println(`Usage: tforge generate [options]

Generate Go SDK code from a Terraform provider schema.

Options:`)
		fs.PrintDefaults()
		fmt.Println(`
Examples:
  # Generate from provider registry
  tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output ./datadog

  # Generate from cached schema
  tforge generate -schema ./schema.json -package aws -output ./aws

  # Use with go:generate
  //go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate inputs
	if *schemaFile == "" && (*providerSource == "" || *providerVersion == "") {
		fmt.Fprintln(os.Stderr, "Error: either -schema or both -provider and -version are required")
		fs.Usage()
		os.Exit(1)
	}

	var schema *provider.Schema
	var err error

	if *schemaFile != "" {
		// Load from file
		fmt.Fprintf(os.Stderr, "Loading schema from %s...\n", *schemaFile)
		schema, err = provider.LoadSchemaFromFile(*schemaFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading schema: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Fetch from registry
		fmt.Fprintf(os.Stderr, "Fetching schema for %s@%s...\n", *providerSource, *providerVersion)
		schema, err = provider.FetchSchema(*providerSource, *providerVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching schema: %v\n", err)
			os.Exit(1)
		}
		schema.Source, _ = provider.ParseSource(*providerSource)
		schema.Version = *providerVersion
	}

	// Determine package name
	pkg := *packageName
	if pkg == "" {
		if schema.Source.Name != "" {
			pkg = schema.Source.Name
		} else {
			pkg = "generated"
		}
	}

	fmt.Fprintf(os.Stderr, "Generating Go code to %s (package %s)...\n", *outputDir, pkg)

	// Generate code
	gen := generate.New(generate.Config{
		OutputDir:        *outputDir,
		PackageName:      pkg,
		GenerateComments: *comments,
		OmitEmpty:        *omitEmpty,
	})

	if err := gen.Generate(schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Generated %d resources and %d data sources\n",
		len(schema.Provider.ResourceSchemas),
		len(schema.Provider.DataSourceSchemas))
}

func schemaCmd(args []string) {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)

	providerSource := fs.String("provider", "", "Provider source (namespace/name), e.g., datadog/datadog")
	providerVersion := fs.String("version", "", "Provider version, e.g., 3.39.0")
	outputFile := fs.String("output", "", "Output file path (default: stdout)")

	fs.Usage = func() {
		fmt.Println(`Usage: tforge schema [options]

Fetch a provider schema and output as JSON.

Options:`)
		fs.PrintDefaults()
		fmt.Println(`
Examples:
  # Save schema to file
  tforge schema -provider datadog/datadog -version 3.39.0 -output datadog-schema.json

  # Print schema to stdout
  tforge schema -provider hashicorp/aws -version 5.0.0`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *providerSource == "" || *providerVersion == "" {
		fmt.Fprintln(os.Stderr, "Error: -provider and -version are required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Fetching schema for %s@%s...\n", *providerSource, *providerVersion)

	schema, err := provider.FetchSchema(*providerSource, *providerVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching schema: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(schema.Provider, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
		os.Exit(1)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Schema saved to %s\n", *outputFile)
	} else {
		fmt.Println(string(data))
	}
}
