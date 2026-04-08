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
	"io"
	"os"

	"tforge/generate"
	"tforge/provider"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

// run is the testable entry point. Returns exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stdout)
		return 1
	}

	switch args[1] {
	case "generate", "gen":
		if err := runGenerate(args[2:], stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "schema":
		if err := runSchema(args[2:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "tforge version %s\n", version)
	case "help", "-h", "--help":
		printUsage(stdout)
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[1])
		printUsage(stderr)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `tforge - Generate Go SDK from Terraform provider schemas

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

func runGenerate(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	providerSource := fs.String("provider", "", "Provider source (namespace/name), e.g., datadog/datadog")
	providerVersion := fs.String("version", "", "Provider version, e.g., 3.39.0")
	schemaFile := fs.String("schema", "", "Path to cached schema JSON file (alternative to -provider/-version)")
	outputDir := fs.String("output", "./generated", "Output directory for generated code")
	packageName := fs.String("package", "", "Go package name (defaults to provider name)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: tforge generate [options]

Generate Go SDK code from a Terraform provider schema.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(stderr, `
Examples:
  # Generate from provider registry
  tforge generate -provider datadog/datadog -version 3.39.0 -package datadog -output ./datadog

  # Generate from cached schema
  tforge generate -schema ./schema.json -package aws -output ./aws

  # Use with go:generate
  //go:generate tforge generate -provider datadog/datadog -version 3.39.0 -package datadog`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *schemaFile == "" && (*providerSource == "" || *providerVersion == "") {
		fs.Usage()
		return fmt.Errorf("either -schema or both -provider and -version are required")
	}

	var schema *provider.Schema
	var err error

	if *schemaFile != "" {
		fmt.Fprintf(stderr, "Loading schema from %s...\n", *schemaFile)
		schema, err = provider.LoadSchemaFromFile(*schemaFile)
		if err != nil {
			return fmt.Errorf("loading schema: %w", err)
		}
	} else {
		fmt.Fprintf(stderr, "Fetching schema for %s@%s...\n", *providerSource, *providerVersion)
		schema, err = provider.FetchSchema(*providerSource, *providerVersion)
		if err != nil {
			return fmt.Errorf("fetching schema: %w", err)
		}
		schema.Source, _ = provider.ParseSource(*providerSource)
		schema.Version = *providerVersion
	}

	pkg := *packageName
	if pkg == "" {
		if schema.Source.Name != "" {
			pkg = schema.Source.Name
		} else {
			pkg = "generated"
		}
	}

	fmt.Fprintf(stderr, "Generating Go code to %s (package %s)...\n", *outputDir, pkg)

	gen := generate.New(generate.Config{
		OutputDir:   *outputDir,
		PackageName: pkg,
	})

	if err := gen.Generate(schema); err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	fmt.Fprintf(stderr, "Generated %d resources and %d data sources\n",
		len(schema.Provider.ResourceSchemas),
		len(schema.Provider.DataSourceSchemas))
	return nil
}

func runSchema(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(stderr)

	providerSource := fs.String("provider", "", "Provider source (namespace/name), e.g., datadog/datadog")
	providerVersion := fs.String("version", "", "Provider version, e.g., 3.39.0")
	outputFile := fs.String("output", "", "Output file path (default: stdout)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: tforge schema [options]

Fetch a provider schema and output as JSON.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(stderr, `
Examples:
  # Save schema to file
  tforge schema -provider datadog/datadog -version 3.39.0 -output datadog-schema.json

  # Print schema to stdout
  tforge schema -provider hashicorp/aws -version 5.0.0`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *providerSource == "" || *providerVersion == "" {
		fs.Usage()
		return fmt.Errorf("-provider and -version are required")
	}

	fmt.Fprintf(stderr, "Fetching schema for %s@%s...\n", *providerSource, *providerVersion)

	schema, err := provider.FetchSchema(*providerSource, *providerVersion)
	if err != nil {
		return fmt.Errorf("fetching schema: %w", err)
	}

	data, err := json.MarshalIndent(schema.Provider, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding schema: %w", err)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Fprintf(stderr, "Schema saved to %s\n", *outputFile)
	} else {
		fmt.Fprintln(stdout, string(data))
	}
	return nil
}
