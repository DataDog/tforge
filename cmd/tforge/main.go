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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"tforge/generate"
	"tforge/provider"
)

const version = "0.2.0"

const (
	dirPerm  = os.FileMode(0755)
	filePerm = os.FileMode(0644)
)

// command represents a CLI subcommand with a typed run function.
type command struct {
	// run executes the subcommand, returning an error on failure.
	run func(args []string, stdout, stderr io.Writer) error
}

// commands maps subcommand names (including aliases) to their implementations.
var commands = map[string]command{
	"generate": {run: func(args []string, stdout, stderr io.Writer) error { return runGenerate(args, stderr) }},
	"gen":      {run: func(args []string, stdout, stderr io.Writer) error { return runGenerate(args, stderr) }},
	"schema":   {run: func(args []string, stdout, stderr io.Writer) error { return runSchema(args, stdout, stderr) }},
	"migrate":  {run: func(args []string, stdout, stderr io.Writer) error { return runMigrate(args, stdout, stderr) }},
	"version": {run: func(args []string, stdout, _ io.Writer) error {
		fmt.Fprintf(stdout, "tforge version %s\n", version)
		return nil
	}},
	"help": {run: func(args []string, stdout, _ io.Writer) error {
		printUsage(stdout)
		return nil
	}},
}

func init() {
	// Register flag-style aliases that map to existing commands.
	commands["-v"] = commands["version"]
	commands["--version"] = commands["version"]
	commands["-h"] = commands["help"]
	commands["--help"] = commands["help"]
}

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

// run is the testable entry point. Returns exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stdout)
		return 1
	}

	cmd, ok := commands[args[1]]
	if !ok {
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[1])
		printUsage(stderr)
		return 1
	}

	if err := cmd.run(args[2:], stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
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
  migrate     Migrate a CDKTF Go project to tforge
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
	outputDir := fs.String("output", generate.DefaultOutputDir, "Output directory for generated code")
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

	var ps *provider.ProviderSchema
	var source provider.Source
	var err error

	if *schemaFile != "" {
		fmt.Fprintf(stderr, "Loading schema from %s...\n", *schemaFile)
		ps, err = provider.LoadSchemaFromFile(*schemaFile)
		if err != nil {
			return fmt.Errorf("loading schema: %w", err)
		}
	} else {
		fmt.Fprintf(stderr, "Fetching schema for %s@%s...\n", *providerSource, *providerVersion)
		schema, fetchErr := provider.FetchSchema(context.Background(), *providerSource, *providerVersion)
		if fetchErr != nil {
			return fmt.Errorf("fetching schema: %w", fetchErr)
		}
		ps = schema.Provider
		source = schema.Source
	}

	pkg := *packageName
	if pkg == "" {
		if source.Name != "" {
			pkg = source.Name
		} else {
			pkg = generate.DefaultPackageName
		}
	}

	fmt.Fprintf(stderr, "Generating Go code to %s (package %s)...\n", *outputDir, pkg)

	gen := generate.New(generate.Config{
		OutputDir:   *outputDir,
		PackageName: pkg,
	})

	if err := gen.Generate(ps); err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	fmt.Fprintf(stderr, "Generated %d resources and %d data sources\n",
		len(ps.ResourceSchemas),
		len(ps.DataSourceSchemas))
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

	schema, err := provider.FetchSchema(context.Background(), *providerSource, *providerVersion)
	if err != nil {
		return fmt.Errorf("fetching schema: %w", err)
	}

	data, err := json.MarshalIndent(schema.Provider, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding schema: %w", err)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, data, filePerm); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Fprintf(stderr, "Schema saved to %s\n", *outputFile)
	} else {
		fmt.Fprintln(stdout, string(data))
	}
	return nil
}
