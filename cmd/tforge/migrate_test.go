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
	if !strings.Contains(out, "aws/generate.go") {
		t.Errorf("expected aws/generate.go in output, got: %s", out)
	}
	if !strings.Contains(out, "modules") {
		t.Errorf("expected module warning in output, got: %s", out)
	}

	// cdktf.json should still exist (dry run)
	if _, err := os.Stat(filepath.Join(dir, "cdktf.json")); err != nil {
		t.Error("cdktf.json should not be deleted in dry-run mode")
	}
	// aws/ subdir should not be created (dry run)
	if _, err := os.Stat(filepath.Join(dir, "aws")); err == nil {
		t.Error("aws/ should not be created in dry-run mode")
	}
}

func TestRunMigrate_GitignoreWriteError(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	// Create .gitignore as a read-only directory to provoke a write error.
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.Mkdir(gitignorePath, 0755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when .gitignore cannot be written, got nil")
	}
	if !strings.Contains(err.Error(), ".gitignore") {
		t.Errorf("expected .gitignore error, got: %v", err)
	}
}

func TestRunMigrate_StringProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("cdktf.out\ngenerated\n"), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
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

	// aws/generate.go created
	genData, err := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
	if err != nil {
		t.Fatalf("aws/generate.go not created: %v", err)
	}
	genContent := string(genData)
	if !strings.Contains(genContent, "//go:generate tforge generate -provider hashicorp/aws -version 5.0 -package aws -output .") {
		t.Errorf("wrong generate directive: %s", genContent)
	}
	if !strings.Contains(genContent, "package aws") {
		t.Errorf("wrong package name: %s", genContent)
	}

	// .gitignore updated (only appends, does not remove existing entries)
	gitignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	gi := string(gitignore)
	if !strings.Contains(gi, "cdktf.out") {
		t.Error(".gitignore should still contain original cdktf.out entry")
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

	genData, _ := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
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

	genData, _ := os.ReadFile(filepath.Join(dir, "random", "generate.go"))
	if !strings.Contains(string(genData), "-version latest") {
		t.Errorf("expected 'latest' version: %s", string(genData))
	}
	if !strings.Contains(string(genData), "package random") {
		t.Errorf("expected package random: %s", string(genData))
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

	// Both provider packages created
	if _, err := os.Stat(filepath.Join(dir, "aws", "generate.go")); err != nil {
		t.Error("aws/generate.go not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "datadog", "generate.go")); err != nil {
		t.Error("datadog/generate.go not created")
	}

	awsGen, _ := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
	if !strings.Contains(string(awsGen), "package aws") {
		t.Error("aws package should be named aws")
	}
	ddGen, _ := os.ReadFile(filepath.Join(dir, "datadog", "generate.go"))
	if !strings.Contains(string(ddGen), "package datadog") {
		t.Error("datadog package should be named datadog")
	}
}

func TestRunMigrate_ThirdPartyProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["kreuzwerker/docker@~> 2.11"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genData, _ := os.ReadFile(filepath.Join(dir, "docker", "generate.go"))
	content := string(genData)
	if !strings.Contains(content, "-provider kreuzwerker/docker") {
		t.Errorf("expected kreuzwerker/docker provider: %s", content)
	}
	if !strings.Contains(content, "package docker") {
		t.Errorf("expected package docker: %s", content)
	}
}

func TestParseProviderString(t *testing.T) {
	tests := []struct {
		input   string
		source  string
		name    string
		version string
	}{
		{"hashicorp/aws@~> 3.70.0", "hashicorp/aws", "aws", "3.70.0"},
		{"hashicorp/aws@>= 5.0", "hashicorp/aws", "aws", "5.0"},
		{"datadog/datadog@~> 3.39", "datadog/datadog", "datadog", "3.39"},
		{"hashicorp/random", "hashicorp/random", "random", "latest"},
		{"aws@~> 5.0", "hashicorp/aws", "aws", "5.0"},
		{"aws", "hashicorp/aws", "aws", "latest"},
		{"kreuzwerker/docker@~> 2.11", "kreuzwerker/docker", "docker", "2.11"},
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
			if p.Name != tt.name {
				t.Errorf("name = %q, want %q", p.Name, tt.name)
			}
			if p.Version != tt.version {
				t.Errorf("version = %q, want %q", p.Version, tt.version)
			}
		})
	}
}
