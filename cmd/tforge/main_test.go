package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage in stdout, got: %s", stdout.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge", "bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command: bogus") {
		t.Errorf("expected unknown command error, got: %s", stderr.String())
	}
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"tforge", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "tforge version") {
		t.Errorf("expected version output, got: %s", stdout.String())
	}
}

func TestRun_VersionFlags(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"tforge", flag}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%s: expected exit code 0, got %d", flag, code)
		}
		if !strings.Contains(stdout.String(), version) {
			t.Errorf("%s: expected version %q in output, got: %s", flag, version, stdout.String())
		}
	}
}

func TestRun_Help(t *testing.T) {
	for _, flag := range []string{"help", "-h", "--help"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"tforge", flag}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%s: expected exit code 0, got %d", flag, code)
		}
		if !strings.Contains(stdout.String(), "Generate Go SDK") {
			t.Errorf("%s: expected usage in output, got: %s", flag, stdout.String())
		}
	}
}

func TestRunGenerate_MissingFlags(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing flags")
	}
	if !strings.Contains(err.Error(), "either -schema or both -provider and -version are required") {
		t.Errorf("expected missing flags error, got: %v", err)
	}
}

func TestRunGenerate_ProviderWithoutVersion(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-provider", "hashicorp/aws"}, &stderr)
	if err == nil {
		t.Fatal("expected error for provider without version")
	}
	if !strings.Contains(err.Error(), "either -schema or both -provider and -version are required") {
		t.Errorf("expected missing version error, got: %v", err)
	}
}

func TestRunGenerate_VersionWithoutProvider(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-version", "1.0.0"}, &stderr)
	if err == nil {
		t.Fatal("expected error for version without provider")
	}
}

func TestRunGenerate_BadSchemaFile(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", "/nonexistent/schema.json"}, &stderr)
	if err == nil {
		t.Fatal("expected error for nonexistent schema file")
	}
	if !strings.Contains(err.Error(), "loading schema") {
		t.Errorf("expected loading error, got: %v", err)
	}
}

func TestRunGenerate_InvalidSchemaFile(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(schemaPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath}, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "loading schema") {
		t.Errorf("expected loading error, got: %v", err)
	}
}

func TestRunGenerate_SchemaFile(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "schema.json")
	outputDir := filepath.Join(tmp, "output")

	// Minimal valid schema
	schema := `{"resource_schemas":{"example_thing":{"version":0,"block":{"attributes":{"name":{"type":"string","required":true}}}}}}`
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath, "-output", outputDir, "-package", "test"}, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify files were generated
	if _, err := os.Stat(filepath.Join(outputDir, "resources_gen.go")); err != nil {
		t.Errorf("expected resources_gen.go to be generated: %v", err)
	}
}

func TestRunGenerate_PackageNameInference(t *testing.T) {
	tmp := t.TempDir()
	schemaPath := filepath.Join(tmp, "schema.json")
	outputDir := filepath.Join(tmp, "output")

	// Schema with no source info — package should default to "generated"
	schema := `{"resource_schemas":{"thing":{"version":0,"block":{"attributes":{"id":{"type":"string","computed":true}}}}}}`
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := runGenerate([]string{"-schema", schemaPath, "-output", outputDir}, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "package generated") {
		t.Errorf("expected package name 'generated' in output, got: %s", stderr.String())
	}
}

func TestRunGenerate_UnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	err := runGenerate([]string{"-bogus"}, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunSchema_MissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing flags")
	}
	if !strings.Contains(err.Error(), "-provider and -version are required") {
		t.Errorf("expected missing flags error, got: %v", err)
	}
}

func TestRunSchema_ProviderWithoutVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{"-provider", "hashicorp/aws"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for provider without version")
	}
}

func TestRunSchema_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runSchema([]string{"-bogus"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
