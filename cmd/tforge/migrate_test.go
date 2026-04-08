package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrate_NoCdktfJson(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing cdktf.json")
	}
	if !strings.Contains(err.Error(), "cdktf.json") {
		t.Errorf("expected cdktf.json error, got: %v", err)
	}
}

func TestRunMigrate_InvalidJson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte("not json"), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRunMigrate_NoProviders(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(`{"terraformProviders":[]}`), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for no providers")
	}
	if !strings.Contains(err.Error(), "no terraformProviders") {
		t.Errorf("expected no providers error, got: %v", err)
	}
}

func TestRunMigrate_DryRun(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{
		"terraformProviders": ["hashicorp/aws@~> 3.70.0"],
		"terraformModules": [{"name":"eks","source":"terraform-aws-modules/eks/aws","version":"~> 19.0"}]
	}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir, "-dry-run"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "hashicorp/aws@3.70.0") {
		t.Errorf("expected provider in output, got: %s", out)
	}
	if !strings.Contains(out, "go:generate") {
		t.Errorf("expected go:generate directive in output, got: %s", out)
	}
	if !strings.Contains(out, "modules") {
		t.Errorf("expected module warning in output, got: %s", out)
	}

	// cdktf.json should still exist (dry run)
	if _, err := os.Stat(filepath.Join(dir, "cdktf.json")); err != nil {
		t.Error("cdktf.json should not be deleted in dry-run mode")
	}
}

func TestRunMigrate_StringProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("cdktf.out\ngenerated\n"), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir, "-package", "infra"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// cdktf.json deleted
	if _, err := os.Stat(filepath.Join(dir, "cdktf.json")); !os.IsNotExist(err) {
		t.Error("cdktf.json should be deleted")
	}

	// package.json deleted
	if _, err := os.Stat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
		t.Error("package.json should be deleted")
	}

	// generate.go created with correct directive
	genData, err := os.ReadFile(filepath.Join(dir, "generate.go"))
	if err != nil {
		t.Fatalf("generate.go not created: %v", err)
	}
	genContent := string(genData)
	if !strings.Contains(genContent, "//go:generate tforge generate -provider hashicorp/aws -version 5.0 -package infra -output .") {
		t.Errorf("wrong generate directive: %s", genContent)
	}
	if !strings.Contains(genContent, "package infra") {
		t.Errorf("wrong package name: %s", genContent)
	}

	// .gitignore updated
	gitignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	gi := string(gitignore)
	for _, line := range strings.Split(gi, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "cdktf.out" {
			t.Error(".gitignore should not contain cdktf.out entry")
		}
		if trimmed == "generated" {
			t.Error(".gitignore should not contain generated entry")
		}
	}
	if !strings.Contains(gi, "*_gen.go") {
		t.Error(".gitignore should contain *_gen.go")
	}
}

func TestRunMigrate_ObjectProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": [{"name":"aws","source":"hashicorp/aws","version":"~> 3.22"}]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genData, _ := os.ReadFile(filepath.Join(dir, "generate.go"))
	if !strings.Contains(string(genData), "-provider hashicorp/aws -version 3.22") {
		t.Errorf("wrong generate directive: %s", string(genData))
	}
}

func TestRunMigrate_ProviderWithoutVersion(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/random"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genData, _ := os.ReadFile(filepath.Join(dir, "generate.go"))
	if !strings.Contains(string(genData), "-version latest") {
		t.Errorf("expected 'latest' version for unversioned provider: %s", string(genData))
	}
}

func TestRunMigrate_MultipleProviders(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0", "datadog/datadog@~> 3.39"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genData, _ := os.ReadFile(filepath.Join(dir, "generate.go"))
	content := string(genData)
	if !strings.Contains(content, "hashicorp/aws") {
		t.Error("missing aws provider directive")
	}
	if !strings.Contains(content, "datadog/datadog") {
		t.Error("missing datadog provider directive")
	}
}

func TestParseProviderString(t *testing.T) {
	tests := []struct {
		input   string
		source  string
		version string
	}{
		{"hashicorp/aws@~> 3.70.0", "hashicorp/aws", "3.70.0"},
		{"hashicorp/aws@>= 5.0", "hashicorp/aws", "5.0"},
		{"datadog/datadog@~> 3.39", "datadog/datadog", "3.39"},
		{"hashicorp/random", "hashicorp/random", "latest"},
		{"aws@~> 5.0", "hashicorp/aws", "5.0"},
		{"aws", "hashicorp/aws", "latest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := parseProviderString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Source != tt.source {
				t.Errorf("source = %q, want %q", p.Source, tt.source)
			}
			if p.Version != tt.version {
				t.Errorf("version = %q, want %q", p.Version, tt.version)
			}
		})
	}
}
