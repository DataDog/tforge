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
	Version string // e.g., "3.70.0" (without constraint prefix)
}

func runMigrate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dir := fs.String("dir", ".", "Directory containing cdktf.json")
	packageName := fs.String("package", "main", "Go package name for generated code")
	outputDir := fs.String("output", ".", "Output directory for generated files")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")

	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: tforge migrate [options]

Migrate a CDKTF Go project to tforge.

Reads cdktf.json, extracts provider requirements, and sets up
go:generate directives for tforge. Removes CDKTF-specific files.

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

	// Generate the go:generate directives
	var directives []string
	for _, p := range providers {
		directive := fmt.Sprintf("//go:generate tforge generate -provider %s -version %s -package %s -output %s",
			p.Source, p.Version, *packageName, *outputDir)
		directives = append(directives, directive)
	}

	if *dryRun {
		fmt.Fprintln(stdout, "Dry run — no changes will be made.")
		fmt.Fprintln(stdout, "Providers found:")
		for _, p := range providers {
			fmt.Fprintf(stdout, "  %s@%s\n", p.Source, p.Version)
		}
		fmt.Fprintln(stdout, "\ngo:generate directives to add:")
		for _, d := range directives {
			fmt.Fprintf(stdout, "  %s\n", d)
		}
		fmt.Fprintln(stdout, "\nFiles to delete:")
		fmt.Fprintf(stdout, "  %s\n", cdktfPath)
		pkgJSON := filepath.Join(*dir, "package.json")
		if _, err := os.Stat(pkgJSON); err == nil {
			fmt.Fprintf(stdout, "  %s\n", pkgJSON)
		}
		printManualSteps(stdout, directives, cfg.TerraformModules)
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

	// Update .gitignore
	if err := updateGitignore(*dir, stderr); err != nil {
		fmt.Fprintf(stderr, "Warning: could not update .gitignore: %v\n", err)
	}

	// Write generate.go with go:generate directives
	generateFile := filepath.Join(*dir, "generate.go")
	var buf strings.Builder
	for _, d := range directives {
		buf.WriteString(d + "\n")
	}
	buf.WriteString("package " + *packageName + "\n")

	fmt.Fprintf(stderr, "Writing %s\n", generateFile)
	if err := os.WriteFile(generateFile, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("writing generate.go: %w", err)
	}

	printManualSteps(stdout, directives, cfg.TerraformModules)

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

		version := stripConstraintPrefix(obj.Version)
		if version == "" {
			version = "latest"
		}

		providers = append(providers, parsedProvider{Source: source, Version: version})
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

	version := "latest"
	if len(parts) == 2 {
		version = stripConstraintPrefix(strings.TrimSpace(parts[1]))
	}

	return parsedProvider{Source: source, Version: version}, nil
}

// stripConstraintPrefix removes version constraint operators like "~> ", ">= ", "= ".
func stripConstraintPrefix(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimLeft(v, "~>=! ")
	return strings.TrimSpace(v)
}

// updateGitignore adds tforge entries and removes cdktf-specific entries.
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

	lines := strings.Split(string(content), "\n")

	// Remove cdktf-specific entries
	cdktfEntries := map[string]bool{
		"cdktf.out":  true,
		"generated":  true,
		"cdktf.out/": true,
		"generated/": true,
	}
	var filtered []string
	for _, line := range lines {
		if !cdktfEntries[strings.TrimSpace(line)] {
			filtered = append(filtered, line)
		}
	}

	// Add tforge entry if not present
	tforgeEntry := "*_gen.go"
	found := false
	for _, line := range filtered {
		if strings.TrimSpace(line) == tforgeEntry {
			found = true
			break
		}
	}
	if !found {
		// Add with a section header
		if len(filtered) > 0 && filtered[len(filtered)-1] != "" {
			filtered = append(filtered, "")
		}
		filtered = append(filtered, "# tforge generated files")
		filtered = append(filtered, tforgeEntry)
	}

	result := strings.Join(filtered, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	fmt.Fprintf(stderr, "Updating %s\n", path)
	return os.WriteFile(path, []byte(result), 0644)
}

func printManualSteps(w io.Writer, directives []string, modules []json.RawMessage) {
	fmt.Fprintln(w, "\n=== Migration Summary ===")
	fmt.Fprintln(w, "\ngo:generate directives written to generate.go:")
	for _, d := range directives {
		fmt.Fprintf(w, "  %s\n", d)
	}

	fmt.Fprintln(w, "\nNext steps:")
	fmt.Fprintln(w, "  1. Update go.mod: add tforge dependency, remove cdktf/jsii/constructs")
	fmt.Fprintln(w, "     go get tforge@latest")
	fmt.Fprintln(w, "  2. Generate provider bindings:")
	fmt.Fprintln(w, "     go generate ./...")
	fmt.Fprintln(w, "  3. Update imports in your Go files:")
	fmt.Fprintln(w, "     - Remove: constructs, jsii-runtime-go, terraform-cdk-go/cdktf")
	fmt.Fprintln(w, "     - Remove: generated provider packages (e.g., generated/hashicorp/aws/*)")
	fmt.Fprintln(w, "     - Add:    tforge/config, tforge/render")
	fmt.Fprintln(w, "  4. Replace CDKTF patterns:")
	fmt.Fprintln(w, "     - cdktf.NewApp / app.Synth()  →  render.ToFile(stack, \"main.tf.json\")")
	fmt.Fprintln(w, "     - cdktf.NewTerraformStack()   →  config.NewStack(\"name\")")
	fmt.Fprintln(w, "     - provider.NewXxx(stack, ...)  →  stack.AddProvider(config.Provider{...})")
	fmt.Fprintln(w, "     - resource.NewXxx(stack, ...)  →  NewXxx(stack, \"name\", XxxConfig{...})")
	fmt.Fprintln(w, "     - jsii.String(\"value\")         →  \"value\"")
	fmt.Fprintln(w, "  5. Run and verify:")
	fmt.Fprintln(w, "     go run . && terraform init && terraform validate")

	if len(modules) > 0 {
		fmt.Fprintln(w, "\n⚠ Warning: This project uses Terraform modules:")
		for _, m := range modules {
			fmt.Fprintf(w, "    %s\n", string(m))
		}
		fmt.Fprintln(w, "  tforge does not support modules. Replace with equivalent resources.")
	}
}
