package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cdktfConfig represents the relevant fields of cdktf.json.
type cdktfConfig struct {
	TerraformProviders []json.RawMessage `json:"terraformProviders"`
	TerraformModules   []json.RawMessage `json:"terraformModules"`
}

// cdktfProvider represents the object form of a provider in cdktf.json.
type cdktfProvider struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version"`
}

// parsedProvider is the normalized result of parsing a provider entry.
type parsedProvider struct {
	Source  string // e.g., "hashicorp/aws"
	Name    string // e.g., "aws" (last part of source, used as package name)
	Version string // e.g., "3.70.0" (without constraint prefix)
}

func runMigrate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dir := fs.String("dir", ".", "Directory containing cdktf.json")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")

	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: tforge migrate [options]

Migrate a CDKTF Go project to tforge.

Reads cdktf.json, extracts provider requirements, and creates a
sub-package per provider with go:generate directives (matching the
tforge convention: each provider gets its own package).

For example, "hashicorp/aws@~> 3.70.0" creates:
  aws/generate.go  with  //go:generate tforge generate -provider hashicorp/aws ...

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(stderr, `
Examples:
  # Migrate current directory
  tforge migrate

  # Migrate a specific directory
  tforge migrate -dir ./my-cdktf-project

  # Preview changes without applying
  tforge migrate -dry-run`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	cdktfPath := filepath.Join(*dir, "cdktf.json")
	data, err := os.ReadFile(cdktfPath)
	if err != nil {
		return fmt.Errorf("reading cdktf.json: %w (is this a CDKTF project?)", err)
	}

	var cfg cdktfConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing cdktf.json: %w", err)
	}

	providers, err := parseProviders(cfg.TerraformProviders)
	if err != nil {
		return fmt.Errorf("parsing providers: %w", err)
	}

	if len(providers) == 0 {
		return fmt.Errorf("no terraformProviders found in cdktf.json")
	}

	if *dryRun {
		fmt.Fprintln(stdout, "Dry run — no changes will be made.")
		fmt.Fprintln(stdout, "\nProvider packages to create:")
		for _, p := range providers {
			fmt.Fprintf(stdout, "  %s/generate.go  (provider %s@%s)\n", p.Name, p.Source, p.Version)
		}
		fmt.Fprintln(stdout, "\nFiles to delete:")
		fmt.Fprintf(stdout, "  %s\n", cdktfPath)
		pkgJSON := filepath.Join(*dir, "package.json")
		if _, err := os.Stat(pkgJSON); err == nil {
			fmt.Fprintf(stdout, "  %s\n", pkgJSON)
		}
		printManualSteps(stdout, providers, cfg.TerraformModules)
		return nil
	}

	// Delete CDKTF files
	fmt.Fprintf(stderr, "Removing %s\n", cdktfPath)
	if err := os.Remove(cdktfPath); err != nil {
		return fmt.Errorf("removing cdktf.json: %w", err)
	}

	pkgJSON := filepath.Join(*dir, "package.json")
	if _, err := os.Stat(pkgJSON); err == nil {
		fmt.Fprintf(stderr, "Removing %s\n", pkgJSON)
		if err := os.Remove(pkgJSON); err != nil {
			return fmt.Errorf("removing package.json: %w", err)
		}
	}

	// Update .gitignore to prevent generated files from being committed.
	if err := updateGitignore(*dir, stderr); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}

	// Create a sub-package per provider with generate.go
	for _, p := range providers {
		pkgDir := filepath.Join(*dir, p.Name)
		if err := os.MkdirAll(pkgDir, dirPerm); err != nil {
			return fmt.Errorf("creating %s/: %w", p.Name, err)
		}

		generateFile := filepath.Join(pkgDir, "generate.go")
		content := fmt.Sprintf(
			"// Package %s provides generated types for the %s Terraform provider.\n"+
				"//\n"+
				"//go:generate tforge generate -provider %s -version %s -package %s -output .\n"+
				"package %s\n",
			p.Name, p.Source, p.Source, p.Version, p.Name, p.Name)

		fmt.Fprintf(stderr, "Writing %s\n", generateFile)
		if err := os.WriteFile(generateFile, []byte(content), filePerm); err != nil {
			return fmt.Errorf("writing %s: %w", generateFile, err)
		}
	}

	printManualSteps(stdout, providers, cfg.TerraformModules)

	return nil
}

// parseProviders parses the terraformProviders array from cdktf.json.
// Supports both string format ("hashicorp/aws@~> 3.70.0") and object format.
func parseProviders(raw []json.RawMessage) ([]parsedProvider, error) {
	var providers []parsedProvider

	for _, entry := range raw {
		// Try string format first: "hashicorp/aws@~> 3.70.0" or "hashicorp/aws"
		var str string
		if err := json.Unmarshal(entry, &str); err == nil {
			p, err := parseProviderString(str)
			if err != nil {
				return nil, err
			}
			providers = append(providers, p)
			continue
		}

		// Try object format: {"name": "aws", "source": "hashicorp/aws", "version": "~> 3.22"}
		var obj cdktfProvider
		if err := json.Unmarshal(entry, &obj); err != nil {
			return nil, fmt.Errorf("unrecognized provider format: %s", string(entry))
		}

		source := obj.Source
		if source == "" && obj.Name != "" {
			source = "hashicorp/" + obj.Name
		}
		if source == "" {
			return nil, fmt.Errorf("provider entry missing source: %s", string(entry))
		}

		name := obj.Name
		if name == "" {
			name = providerName(source)
		}

		version := stripConstraintPrefix(obj.Version)
		if version == "" {
			version = "latest"
		}

		providers = append(providers, parsedProvider{Source: source, Name: name, Version: version})
	}

	return providers, nil
}

// parseProviderString parses "hashicorp/aws@~> 3.70.0" or "hashicorp/aws".
func parseProviderString(s string) (parsedProvider, error) {
	s = strings.TrimSpace(s)

	parts := strings.SplitN(s, "@", 2)
	source := strings.TrimSpace(parts[0])

	if source == "" {
		return parsedProvider{}, fmt.Errorf("empty provider source: %q", s)
	}

	// If no namespace, assume hashicorp
	if !strings.Contains(source, "/") {
		source = "hashicorp/" + source
	}

	name := providerName(source)

	version := "latest"
	if len(parts) == 2 {
		version = stripConstraintPrefix(strings.TrimSpace(parts[1]))
	}

	return parsedProvider{Source: source, Name: name, Version: version}, nil
}

// providerName extracts the short name from a provider source (e.g., "aws" from "hashicorp/aws").
func providerName(source string) string {
	parts := strings.Split(source, "/")
	return parts[len(parts)-1]
}

// stripConstraintPrefix removes version constraint operators like "~> ", ">= ", "= ".
func stripConstraintPrefix(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimLeft(v, "~>=! ")
	return strings.TrimSpace(v)
}

// updateGitignore appends tforge entries to .gitignore if not already present.
func updateGitignore(dir string, stderr io.Writer) error {
	path := filepath.Join(dir, ".gitignore")

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			content = nil
		} else {
			return err
		}
	}

	// Check if tforge entry already present
	tforgeEntry := "*_gen.go"
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == tforgeEntry {
			return nil // already present
		}
	}

	// Append tforge section
	var addition string
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		addition += "\n"
	}
	addition += "\n# tforge generated files\n" + tforgeEntry + "\n"

	fmt.Fprintf(stderr, "Updating %s\n", path)
	return os.WriteFile(path, append(content, []byte(addition)...), filePerm)
}

func printManualSteps(w io.Writer, providers []parsedProvider, modules []json.RawMessage) {
	fmt.Fprintln(w, "\n=== Migration Summary ===")
	fmt.Fprintln(w, "\nProvider packages created:")
	for _, p := range providers {
		fmt.Fprintf(w, "  %s/  (%s@%s)\n", p.Name, p.Source, p.Version)
	}

	fmt.Fprintln(w, "\nNext steps:")
	fmt.Fprintln(w, "  1. Update go.mod: add tforge dependency, remove cdktf/jsii/constructs")
	fmt.Fprintln(w, "     go get tforge@latest")
	fmt.Fprintln(w, "  2. Generate provider bindings:")
	fmt.Fprintln(w, "     go generate ./...")
	fmt.Fprintln(w, "  3. Update imports in your Go files:")
	fmt.Fprintln(w, "     - Remove: constructs, jsii-runtime-go, terraform-cdk-go/cdktf")
	fmt.Fprintln(w, "     - Remove: generated provider packages (e.g., generated/hashicorp/aws/*)")

	if len(providers) > 0 {
		fmt.Fprintln(w, "     - Add:    tforge/config, tforge/render")
		for _, p := range providers {
			fmt.Fprintf(w, "     - Add:    <your-module>/%s  (generated provider types)\n", p.Name)
		}
	}

	fmt.Fprintln(w, "  4. Replace CDKTF patterns:")
	fmt.Fprintln(w, "     - cdktf.NewApp / app.Synth()  →  render.ToFile(stack, \"main.tf.json\", \"\")")
	fmt.Fprintln(w, "     - cdktf.NewTerraformStack()   →  config.NewStack(\"name\")")
	fmt.Fprintln(w, "     - provider.NewXxx(stack, ...)  →  stack.AddProvider(config.Provider{...})")
	fmt.Fprintln(w, "     - resource.NewXxx(stack, ...)  →  pkg.NewXxx(stack, \"name\", pkg.XxxConfig{...})")
	fmt.Fprintln(w, "     - jsii.String(\"value\")         →  \"value\"")
	fmt.Fprintln(w, "  5. Run and verify:")
	fmt.Fprintln(w, "     go run . && terraform init && terraform validate")

	if len(modules) > 0 {
		fmt.Fprintln(w, "\nWarning: This project uses Terraform modules:")
		for _, m := range modules {
			fmt.Fprintf(w, "    %s\n", string(m))
		}
		fmt.Fprintln(w, "  tforge does not support modules. Replace with equivalent resources.")
	}
}
