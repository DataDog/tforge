// Package provider handles fetching OpenTofu providers and loading their schemas.
//
// This package is responsible for:
//   - Creating temporary OpenTofu configurations
//   - Running `tofu providers schema -json` to extract schemas
//   - Returning provider schemas as structured Go types
//
// Example usage:
//
//	schema, err := provider.FetchSchema("datadog/datadog", "3.39.0")
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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	intzip "tforge/internal/zip"
)

// RegistryURL is the base URL for the OpenTofu/Terraform registry API.
const RegistryURL = "https://registry.opentofu.org"

// FallbackRegistryURL is the Terraform registry used as fallback.
const FallbackRegistryURL = "https://registry.terraform.io"

// ErrProviderNotFound indicates the requested provider does not exist.
var ErrProviderNotFound = errors.New("provider not found")

// ErrVersionNotFound indicates the requested version does not exist.
var ErrVersionNotFound = errors.New("version not found")

// ErrCLINotFound indicates neither tofu nor terraform binary was found.
var ErrCLINotFound = errors.New("neither tofu nor terraform binary found in PATH")

// findCLI finds the tofu or terraform binary, preferring tofu.
func findCLI() (string, error) {
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

// Source represents a provider source address (e.g., "datadog/datadog").
type Source struct {
	Namespace string // e.g., "datadog"
	Name      string // e.g., "datadog"
}

// String returns the full source address.
func (s Source) String() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

// ParseSource parses a provider source string like "datadog/datadog".
func ParseSource(src string) (Source, error) {
	parts := strings.SplitN(src, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Source{}, fmt.Errorf("invalid provider source: %q (expected namespace/name)", src)
	}
	return Source{Namespace: parts[0], Name: parts[1]}, nil
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

// NestedType represents a nested attribute type.
type NestedType struct {
	Attributes  map[string]*Attribute `json:"attributes,omitempty"`
	NestingMode string                `json:"nesting_mode,omitempty"`
}

// NestedBlock represents a nested block type.
type NestedBlock struct {
	Block       *BlockType `json:"block,omitempty"`
	NestingMode string     `json:"nesting_mode,omitempty"`
	MinItems    int        `json:"min_items,omitempty"`
	MaxItems    int        `json:"max_items,omitempty"`
}

// FetchSchema fetches a provider schema using tofu CLI.
// The source should be in the format "namespace/name" (e.g., "datadog/datadog").
func FetchSchema(source, version string) (*Schema, error) {
	return FetchSchemaContext(context.Background(), source, version)
}

// FetchSchemaContext fetches a provider schema with context.
func FetchSchemaContext(ctx context.Context, source, version string) (*Schema, error) {
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

	// Write minimal tofu configuration
	tfConfig := fmt.Sprintf(`
terraform {
  required_providers {
    %s = {
      source  = "%s/%s"
      version = "%s"
    }
  }
}

provider "%s" {}
`, src.Name, src.Namespace, src.Name, version, src.Name)

	configPath := filepath.Join(tmpDir, "main.tf")
	if err := os.WriteFile(configPath, []byte(tfConfig), 0644); err != nil {
		return nil, fmt.Errorf("writing config: %w", err)
	}

	// Run init
	initCmd := exec.CommandContext(ctx, cliPath, "init", "-input=false", "-no-color")
	initCmd.Dir = tmpDir
	initOut, err := initCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("init failed: %w\n%s", err, initOut)
	}

	// Run providers schema -json
	schemaCmd := exec.CommandContext(ctx, cliPath, "providers", "schema", "-json")
	schemaCmd.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	schemaCmd.Stdout = &stdout
	schemaCmd.Stderr = &stderr
	if err := schemaCmd.Run(); err != nil {
		return nil, fmt.Errorf("providers schema failed: %w\n%s", err, stderr.String())
	}

	// Parse the schema JSON
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

// parseSchemaOutput extracts the provider schema from tofu output.
func parseSchemaOutput(data []byte, src Source) (*ProviderSchema, error) {
	var output schemaOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("unmarshaling schema: %w", err)
	}

	// Find the provider schema (key format: "registry.opentofu.org/namespace/name")
	for key, ps := range output.ProviderSchemas {
		if strings.Contains(key, src.String()) || strings.HasSuffix(key, "/"+src.Name) {
			return &ProviderSchema{
				ConfigSchema:      ps.Provider,
				ResourceSchemas:   ps.ResourceSchemas,
				DataSourceSchemas: ps.DataSourceSchemas,
			}, nil
		}
	}

	return nil, fmt.Errorf("provider %s not found in schema output", src)
}

// FetchSchemaJSON returns the provider schema as raw JSON bytes.
func FetchSchemaJSON(source, version string) ([]byte, error) {
	schema, err := FetchSchema(source, version)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(schema.Provider, "", "  ")
}

// LoadSchemaFromFile loads a provider schema from a JSON file.
func LoadSchemaFromFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}
	return LoadSchemaFromJSON(data)
}

// LoadSchemaFromJSON parses a provider schema from JSON bytes.
func LoadSchemaFromJSON(data []byte) (*Schema, error) {
	var ps ProviderSchema
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}
	return &Schema{Provider: &ps}, nil
}

// SaveSchemaToFile saves a provider schema to a JSON file.
func SaveSchemaToFile(schema *Schema, path string) error {
	data, err := json.MarshalIndent(schema.Provider, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing schema file: %w", err)
	}
	return nil
}

// CacheDir returns the directory used to cache provider data.
func CacheDir() string {
	if dir := os.Getenv("TFORGE_CACHE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tforge", "cache")
}

// ListVersions returns available versions for a provider from the registry.
func ListVersions(ctx context.Context, source string) ([]string, error) {
	return listVersionsWithFallback(ctx, source, RegistryURL, FallbackRegistryURL)
}

// listVersionsWithFallback tries the primary registry first, then falls back
// to the secondary registry. Extracted for testability.
func listVersionsWithFallback(ctx context.Context, source, primaryURL, fallbackURL string) ([]string, error) {
	src, err := ParseSource(source)
	if err != nil {
		return nil, err
	}

	// Try primary registry first, then fallback
	versions, err := listVersionsFromRegistry(ctx, primaryURL, src)
	if err != nil {
		versions, err = listVersionsFromRegistry(ctx, fallbackURL, src)
		if err != nil {
			return nil, err
		}
	}
	return versions, nil
}

func listVersionsFromRegistry(ctx context.Context, registryURL string, src Source) ([]string, error) {
	url := fmt.Sprintf("%s/v1/providers/%s/%s/versions", registryURL, src.Namespace, src.Name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrProviderNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry error: %s", resp.Status)
	}

	var result struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	versions := make([]string, len(result.Versions))
	for i, v := range result.Versions {
		versions[i] = v.Version
	}
	return versions, nil
}

// DownloadProvider downloads a provider binary to the cache.
// This is useful for advanced use cases; FetchSchema handles this automatically.
func DownloadProvider(ctx context.Context, source, version string) (string, error) {
	src, err := ParseSource(source)
	if err != nil {
		return "", err
	}

	// Try OpenTofu registry first
	downloadURL, err := getProviderDownloadURL(ctx, RegistryURL, src, version)
	if err != nil {
		// Fallback to Terraform registry
		downloadURL, err = getProviderDownloadURL(ctx, FallbackRegistryURL, src, version)
		if err != nil {
			return "", fmt.Errorf("getting download URL: %w", err)
		}
	}

	return downloadProviderBinary(ctx, downloadURL, src, version)
}

func getProviderDownloadURL(ctx context.Context, registryURL string, src Source, version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	url := fmt.Sprintf("%s/v1/providers/%s/%s/%s/download/%s/%s",
		registryURL, src.Namespace, src.Name, version, osName, arch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrProviderNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registry error: %s: %s", resp.Status, body)
	}

	var result struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding registry response: %w", err)
	}

	return result.DownloadURL, nil
}

func downloadProviderBinary(ctx context.Context, url string, src Source, version string) (string, error) {
	cacheDir := CacheDir()
	providerDir := filepath.Join(cacheDir, "providers", src.Namespace, src.Name, version)

	// Check if already cached
	binaryName := fmt.Sprintf("terraform-provider-%s_v%s", src.Name, version)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(providerDir, binaryName)

	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil // Already cached
	}

	// Create cache directory
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}

	// Download ZIP
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	// Save to temp file
	tmpFile, err := os.CreateTemp(providerDir, "provider-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Extract ZIP
	if err := intzip.Extract(tmpPath, providerDir); err != nil {
		return "", fmt.Errorf("extracting provider: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return "", fmt.Errorf("chmod provider binary: %w", err)
	}

	return binaryPath, nil
}
