package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge"}, &stdout, &stderr)
	require.Equal(t, 1, code, "expected exit code 1, got %d", code)
	require.Contains(t, stdout.String(), "Usage:", "expected usage in stdout, got: %s", stdout.String())
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge", "bogus"}, &stdout, &stderr)
	require.Equal(t, 1, code, "expected exit code 1, got %d", code)
	require.Contains(t, stderr.String(), "Unknown command: bogus", "expected unknown command error, got: %s", stderr.String())
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge", "version"}, &stdout, &stderr)
	require.Equal(t, 0, code, "expected exit code 0, got %d", code)
	require.Contains(t, stdout.String(), "tforge version", "expected version output, got: %s", stdout.String())
}

func TestRun_VersionFlags(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"tforge", flag}, &stdout, &stderr)
		require.Equal(t, 0, code, "%s: expected exit code 0, got %d", flag, code)
		require.Contains(t, stdout.String(), version, "%s: expected version %q in output, got: %s", flag, version, stdout.String())
	}
}

func TestRun_Help(t *testing.T) {
	for _, flag := range []string{"help", "-h", "--help"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"tforge", flag}, &stdout, &stderr)
		require.Equal(t, 0, code, "%s: expected exit code 0, got %d", flag, code)
		require.Contains(t, stdout.String(), "Generate Go SDK", "%s: expected usage in output, got: %s", flag, stdout.String())
	}
}

func TestRunGenerate_MissingFlags(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{}, &stderr)
	require.Error(t, err, "expected error for missing flags")
	require.Contains(t, err.Error(), "either -schema or both -provider and -version are required", "expected missing flags error, got: %v", err)
}

func TestRunGenerate_ProviderWithoutVersion(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-provider", "hashicorp/aws"}, &stderr)
	require.Error(t, err, "expected error for provider without version")
	require.Contains(t, err.Error(), "either -schema or both -provider and -version are required", "expected missing version error, got: %v", err)
}

func TestRunGenerate_VersionWithoutProvider(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-version", "1.0.0"}, &stderr)
	require.Error(t, err, "expected error for version without provider")
}

func TestRunGenerate_BadSchemaFile(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", "/nonexistent/schema.json"}, &stderr)
	require.Error(t, err, "expected error for nonexistent schema file")
	require.Contains(t, err.Error(), "loading schema", "expected loading error, got: %v", err)
}

func TestRunGenerate_InvalidSchemaFile(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "bad.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte("not json"), 0644))

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath}, &stderr)
	require.Error(t, err, "expected error for invalid JSON")
	require.Contains(t, err.Error(), "loading schema", "expected loading error, got: %v", err)
}

func TestRunGenerate_SchemaFile(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "schema.json")
	outputDir := filepath.Join(tmp, "output")

	// Minimal valid schema
	schema := `{"resource_schemas":{"example_thing":{"version":0,"block":{"attributes":{"name":{"type":"string","required":true}}}}}}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath, "-output", outputDir, "-package", "test"}, &stderr)
	require.NoError(t, err, "unexpected error")

	// Verify files were generated
	_, err = os.Stat(filepath.Join(outputDir, "resources_gen.go"))
	require.NoError(t, err, "expected resources_gen.go to be generated")
}

func TestRunGenerate_PackageNameInference(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "schema.json")
	outputDir := filepath.Join(tmp, "output")

	// Schema with no source info — package should default to "generated"
	schema := `{"resource_schemas":{"thing":{"version":0,"block":{"attributes":{"id":{"type":"string","computed":true}}}}}}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath, "-output", outputDir}, &stderr)
	require.NoError(t, err, "unexpected error")

	require.Contains(t, stderr.String(), "package generated", "expected package name 'generated' in output, got: %s", stderr.String())
}

func TestRunGenerate_UnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-bogus"}, &stderr)
	require.Error(t, err, "expected error for unknown flag")
}

func TestRunSchema_MissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{}, &stdout, &stderr)
	require.Error(t, err, "expected error for missing flags")
	require.Contains(t, err.Error(), "-provider and -version are required", "expected missing flags error, got: %v", err)
}

func TestRunSchema_ProviderWithoutVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{"-provider", "hashicorp/aws"}, &stdout, &stderr)
	require.Error(t, err, "expected error for provider without version")
}

func TestRunSchema_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{"-bogus"}, &stdout, &stderr)
	require.Error(t, err, "expected error for unknown flag")
}

func TestRunGenerate_ProviderZipFileWithoutProvider(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-provider-zip-file", "/tmp/foo.zip"}, &stderr)
	require.Error(t, err, "expected error for -provider-zip-file without -provider")
	require.Contains(t, err.Error(), "-provider is required")
}

func TestRunGenerate_ProviderZipFileWithVersion(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{
		"-provider-zip-file", "/tmp/foo.zip",
		"-provider", "datadog/datadog",
		"-version", "3.39.0",
	}, &stderr)
	require.Error(t, err, "expected error for -provider-zip-file with -version")
	require.Contains(t, err.Error(), "-version is incompatible")
}

func TestRunGenerate_ProviderZipFileWithSchema(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{
		"-provider-zip-file", "/tmp/foo.zip",
		"-provider", "datadog/datadog",
		"-schema", "/tmp/schema.json",
	}, &stderr)
	require.Error(t, err, "expected error for -provider-zip-file with -schema")
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestRunSchema_ProviderZipFileWithoutProvider(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{"-provider-zip-file", "/tmp/foo.zip"}, &stdout, &stderr)
	require.Error(t, err, "expected error for -provider-zip-file without -provider")
	require.Contains(t, err.Error(), "-provider is required")
}

func TestRunSchema_ProviderZipFileWithVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{
		"-provider-zip-file", "/tmp/foo.zip",
		"-provider", "datadog/datadog",
		"-version", "3.39.0",
	}, &stdout, &stderr)
	require.Error(t, err, "expected error for -provider-zip-file with -version")
	require.Contains(t, err.Error(), "-version is incompatible")
}
