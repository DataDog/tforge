// Package provider handles fetching OpenTofu providers and loading their schemas.
//
// This package is responsible for:
//   - Creating temporary OpenTofu configurations
//   - Running `tofu providers schema -json` to extract schemas
//   - Returning provider schemas as structured Go types
//
// Example usage:
//
//	schema, err := provider.FetchSchema(context.Background(), "datadog/datadog", "3.39.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use schema for code generation
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	dirPerm  = os.FileMode(0755)
	filePerm = os.FileMode(0644)
)

// ErrCLINotFound indicates neither tofu nor terraform binary was found.
var ErrCLINotFound = errors.New("neither tofu nor terraform binary found in PATH")

// findCLI finds the tofu or terraform binary.
// If TFORGE_CLI is set to "tofu" or "terraform", only that binary is used.
// Otherwise, tofu is preferred with terraform as fallback.
func findCLI() (string, error) {
	if forced := os.Getenv("TFORGE_CLI"); forced != "" {
		path, err := exec.LookPath(forced)
		if err != nil {
			return "", fmt.Errorf("%s not found in PATH (set via TFORGE_CLI)", forced)
		}
		return path, nil
	}
	// Prefer tofu
	if path, err := exec.LookPath("tofu"); err == nil {
		return path, nil
	}
	// Fallback to terraform
	if path, err := exec.LookPath("terraform"); err == nil {
		return path, nil
	}
	return "", ErrCLINotFound
}

// Source represents a Terraform provider source address. Two forms are
// recognized, matching the syntax in Terraform's `required_providers` block:
//
//   - `<namespace>/<name>` for the default registry (registry.terraform.io),
//     e.g. "hashicorp/aws", "datadog/datadog".
//   - `<hostname>/<namespace>/<name>` for an explicit registry, e.g.
//     "registry.opentofu.org/hashicorp/aws" or
//     "mirror.corp.internal/datadog/datadog".
//
// For default-registry sources Hostname is left empty and String() returns
// just "namespace/name", preserving the original two-segment behavior for
// existing consumers (dev_overrides keys, required_providers source fields,
// etc.). For hostname-qualified sources String() returns the full
// three-segment address.
type Source struct {
	// Hostname is optional. When empty the source resolves through
	// registry.terraform.io. When set it is the explicit registry host.
	Hostname string

	// Namespace is the provider publisher's namespace (e.g. "hashicorp").
	Namespace string

	// Name is the provider type (e.g. "aws"). This is the local name that
	// Terraform binds resources to by default (e.g. `aws_instance` -> the
	// provider named `aws`), regardless of which Hostname/Namespace served
	// the schema.
	Name string
}

// String returns the full source address. Two-segment form when Hostname is
// empty (the historic shape), three-segment form when Hostname is set.
func (s Source) String() string {
	if s.Hostname != "" {
		return fmt.Sprintf("%s/%s/%s", s.Hostname, s.Namespace, s.Name)
	}
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

// ParseSource parses a Terraform provider source string. Both
// `<namespace>/<name>` (default registry) and
// `<hostname>/<namespace>/<name>` (explicit registry) are accepted. Returns
// an error for any other shape, including extra slashes or empty segments.
func ParseSource(src string) (Source, error) {
	parts := strings.Split(src, "/")
	switch len(parts) {
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return Source{}, invalidSourceError(src)
		}
		return Source{Namespace: parts[0], Name: parts[1]}, nil
	case 3:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return Source{}, invalidSourceError(src)
		}
		return Source{Hostname: parts[0], Namespace: parts[1], Name: parts[2]}, nil
	default:
		return Source{}, invalidSourceError(src)
	}
}

func invalidSourceError(src string) error {
	return fmt.Errorf("invalid provider source: %q (expected namespace/name or hostname/namespace/name)", src)
}

// Schema wraps provider schema with metadata.
type Schema struct {
	Source   Source
	Version  string
	Provider *ProviderSchema
}

// ProviderSchema represents the full schema for a provider.
type ProviderSchema struct {
	ConfigSchema      *SchemaBlock            `json:"provider,omitempty"`
	ResourceSchemas   map[string]*SchemaBlock `json:"resource_schemas,omitempty"`
	DataSourceSchemas map[string]*SchemaBlock `json:"data_source_schemas,omitempty"`
}

// SchemaBlock represents a block in the schema (provider config, resource, or nested block).
type SchemaBlock struct {
	Version     int        `json:"version,omitempty"`
	Block       *BlockType `json:"block,omitempty"`
	Description string     `json:"description,omitempty"`
}

// BlockType contains attributes and nested blocks.
type BlockType struct {
	Attributes      map[string]*Attribute   `json:"attributes,omitempty"`
	BlockTypes      map[string]*NestedBlock `json:"block_types,omitempty"`
	Description     string                  `json:"description,omitempty"`
	DescriptionKind string                  `json:"description_kind,omitempty"`
	Deprecated      bool                    `json:"deprecated,omitempty"`
}

// Attribute represents a schema attribute.
type Attribute struct {
	Type            json.RawMessage `json:"type,omitempty"`
	NestedType      *NestedType     `json:"nested_type,omitempty"`
	Description     string          `json:"description,omitempty"`
	DescriptionKind string          `json:"description_kind,omitempty"`
	Required        bool            `json:"required,omitempty"`
	Optional        bool            `json:"optional,omitempty"`
	Computed        bool            `json:"computed,omitempty"`
	Sensitive       bool            `json:"sensitive,omitempty"`
	Deprecated      bool            `json:"deprecated,omitempty"`
}

// NestingMode describes how a nested block or nested attribute is repeated.
// The underlying type is string so JSON unmarshaling works transparently.
type NestingMode string

const (
	// NestingModeSingle represents a single nested block or attribute.
	NestingModeSingle NestingMode = "single"

	// NestingModeList represents a list of nested blocks or attributes.
	NestingModeList NestingMode = "list"

	// NestingModeSet represents a set of nested blocks or attributes.
	NestingModeSet NestingMode = "set"

	// NestingModeMap represents a map of nested blocks or attributes.
	NestingModeMap NestingMode = "map"

	// NestingModeGroup represents a group nested block.
	NestingModeGroup NestingMode = "group"
)

// NestedType represents a nested attribute type.
type NestedType struct {
	Attributes  map[string]*Attribute `json:"attributes,omitempty"`
	NestingMode NestingMode           `json:"nesting_mode,omitempty"`
}

// NestedBlock represents a nested block type.
type NestedBlock struct {
	Block       *BlockType  `json:"block,omitempty"`
	NestingMode NestingMode `json:"nesting_mode,omitempty"`
	MinItems    int         `json:"min_items,omitempty"`
	MaxItems    int         `json:"max_items,omitempty"`
}

// FetchSchema fetches a provider schema using tofu CLI. The source accepts
// either form documented on Source: "namespace/name" (default registry) or
// "hostname/namespace/name" (explicit registry).
func FetchSchema(ctx context.Context, source, version string) (*Schema, error) {
	src, err := ParseSource(source)
	if err != nil {
		return nil, err
	}

	// Check if tofu or terraform is available (prefer tofu)
	cliPath, err := findCLI()
	if err != nil {
		return nil, err
	}

	// Create temporary directory for tofu config
	tmpDir, err := os.MkdirTemp("", "tforge-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := writeTerraformConfig(tmpDir, src, version); err != nil {
		return nil, err
	}

	// Set up provider plugin cache to avoid re-downloading providers
	cacheDir := "/tmp/tforge-cache"
	if err := os.MkdirAll(cacheDir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	// Build a clean env: inherit parent env, filter out vars that conflict
	// with OpenTofu (e.g. OTEL_TRACES_EXPORTER), and add our overrides.
	cmdEnv := filterEnv(os.Environ(), "OTEL_TRACES_EXPORTER")
	cmdEnv = append(cmdEnv, "TF_PLUGIN_CACHE_DIR="+cacheDir)

	// Run init
	initCmd := exec.CommandContext(ctx, cliPath, "init", "-input=false", "-no-color")
	initCmd.Dir = tmpDir
	initCmd.Env = cmdEnv
	initOut, err := initCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("init failed: %w\n%s", err, initOut)
	}

	return runProvidersSchema(ctx, cliPath, tmpDir, cmdEnv, src, version)
}

// writeTerraformConfig writes a minimal `main.tf` declaring src@version into dir.
func writeTerraformConfig(dir string, src Source, version string) error {
	tfConfig := fmt.Sprintf(`
terraform {
  required_providers {
    %s = {
      source  = "%s"
      version = "%s"
    }
  }
}

provider "%s" {}
`, src.Name, src.String(), version, src.Name)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(tfConfig), filePerm); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// runProvidersSchema runs `tofu providers schema -json` in workDir and parses
// the schema for src. workDir must already contain a valid `main.tf`.
func runProvidersSchema(ctx context.Context, cliPath, workDir string, env []string, src Source, version string) (*Schema, error) {
	schemaCmd := exec.CommandContext(ctx, cliPath, "providers", "schema", "-json")
	schemaCmd.Dir = workDir
	schemaCmd.Env = env
	var stdout, stderr bytes.Buffer
	schemaCmd.Stdout = &stdout
	schemaCmd.Stderr = &stderr
	if err := schemaCmd.Run(); err != nil {
		return nil, fmt.Errorf("providers schema failed: %w\n%s", err, stderr.String())
	}

	providerSchema, err := parseSchemaOutput(stdout.Bytes(), src)
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	return &Schema{
		Source:   src,
		Version:  version,
		Provider: providerSchema,
	}, nil
}

// schemaOutput is the raw output from `tofu providers schema -json`.
type schemaOutput struct {
	FormatVersion   string `json:"format_version"`
	ProviderSchemas map[string]struct {
		Provider          *SchemaBlock            `json:"provider,omitempty"`
		ResourceSchemas   map[string]*SchemaBlock `json:"resource_schemas,omitempty"`
		DataSourceSchemas map[string]*SchemaBlock `json:"data_source_schemas,omitempty"`
	} `json:"provider_schemas"`
}

// providerKeyMatchesSource reports whether a fully-qualified Terraform
// provider key from `tofu providers schema -json` (e.g.
// "registry.terraform.io/hashicorp/aws") refers to src. A two-segment src
// matches any hostname; a three-segment src requires the hostname to match
// exactly.
func providerKeyMatchesSource(key string, src Source) bool {
	keySrc, ok := parseProviderKey(key, src.Hostname)
	if !ok {
		return false
	}
	return src.Namespace == keySrc.Namespace && src.Name == keySrc.Name
}

// providerKeyHasName is the relaxed second-pass match used when the
// namespace doesn't need to align (forks, mirrors). It still pins the
// hostname when src has one set.
func providerKeyHasName(key string, src Source) bool {
	keySrc, ok := parseProviderKey(key, src.Hostname)
	if !ok {
		return false
	}
	return src.Name == keySrc.Name
}

// parseProviderKey parses a schema-output key into a Source and applies the
// hostname filter shared by both lookup passes. Returns ok=false for any
// malformed key or any key whose hostname doesn't match requireHostname
// (when non-empty).
func parseProviderKey(key, requireHostname string) (Source, bool) {
	keySrc, err := ParseSource(key)
	if err != nil {
		return Source{}, false
	}
	if requireHostname != "" && !strings.EqualFold(strings.TrimSuffix(keySrc.Hostname, ":443"), strings.TrimSuffix(requireHostname, ":443")) {
		return Source{}, false
	}
	return keySrc, true
}

// parseSchemaOutput extracts the provider schema from tofu output.
// It prefers an exact namespace/name match and only falls back to a suffix
// match on the provider name when no exact match exists. An ambiguous suffix
// match (more than one candidate) returns an error instead of silently
// picking a random provider. When src has a Hostname set, both passes
// require the key's hostname to match — a registry-pinned source never
// silently resolves to a provider on a different host.
func parseSchemaOutput(data []byte, src Source) (*ProviderSchema, error) {
	var output schemaOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("unmarshaling schema: %w", err)
	}

	// First pass: exact match on namespace/name (and hostname, if set).
	for key, ps := range output.ProviderSchemas {
		if providerKeyMatchesSource(key, src) {
			return &ProviderSchema{
				ConfigSchema:      ps.Provider,
				ResourceSchemas:   ps.ResourceSchemas,
				DataSourceSchemas: ps.DataSourceSchemas,
			}, nil
		}
	}

	// Second pass: fall back to a name-only match. Still pins the hostname
	// when src has one. Collect all candidates to detect ambiguity.
	type candidate struct {
		key string
		ps  *ProviderSchema
	}
	var candidates []candidate
	for key, ps := range output.ProviderSchemas {
		if providerKeyHasName(key, src) {
			candidates = append(candidates, candidate{
				key: key,
				ps: &ProviderSchema{
					ConfigSchema:      ps.Provider,
					ResourceSchemas:   ps.ResourceSchemas,
					DataSourceSchemas: ps.DataSourceSchemas,
				},
			})
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0].ps, nil
	case 0:
		return nil, fmt.Errorf("provider %s not found in schema output", src)
	default:
		keys := make([]string, len(candidates))
		for i, c := range candidates {
			keys[i] = c.key
		}
		return nil, fmt.Errorf("provider %s matched multiple schemas: %s", src, strings.Join(keys, ", "))
	}
}

// LoadSchemaFromFile loads a provider schema from a JSON file.
func LoadSchemaFromFile(path string) (*ProviderSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}
	return LoadSchemaFromJSON(data)
}

// filterEnv returns a copy of env with entries matching any of the given keys removed.
func filterEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, k := range keys {
			if strings.HasPrefix(e, k+"=") {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}

// LoadSchemaFromJSON parses a provider schema from JSON bytes.
func LoadSchemaFromJSON(data []byte) (*ProviderSchema, error) {
	var ps ProviderSchema
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}
	return &ps, nil
}
