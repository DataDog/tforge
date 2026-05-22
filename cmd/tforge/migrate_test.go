package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunMigrate_NoCdktfJson(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.Error(t, err, "expected error for missing cdktf.json")
	require.Contains(t, err.Error(), "cdktf.json", "expected cdktf.json error, got: %v", err)
}

func TestRunMigrate_InvalidJson(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte("not json"), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.Error(t, err, "expected error for invalid JSON")
}

func TestRunMigrate_NoProviders(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(`{"terraformProviders":[]}`), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.Error(t, err, "expected error for no providers")
	require.Contains(t, err.Error(), "no terraformProviders", "expected no providers error, got: %v", err)
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
	require.NoError(t, err, "unexpected error")

	out := stdout.String()
	require.Contains(t, out, "aws/generate.go", "expected aws/generate.go in output, got: %s", out)
	require.Contains(t, out, "modules", "expected module warning in output, got: %s", out)

	// cdktf.json should still exist (dry run)
	_, err = os.Stat(filepath.Join(dir, "cdktf.json"))
	require.NoError(t, err, "cdktf.json should not be deleted in dry-run mode")
	// aws/ subdir should not be created (dry run)
	_, err = os.Stat(filepath.Join(dir, "aws"))
	require.True(t, os.IsNotExist(err), "aws/ should not be created in dry-run mode")
}

func TestRunMigrate_GitignoreWriteError(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	// Create .gitignore as a read-only directory to provoke a write error.
	gitignorePath := filepath.Join(dir, ".gitignore")
	require.NoError(t, os.Mkdir(gitignorePath, 0755))

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.Error(t, err, "expected error when .gitignore cannot be written, got nil")
	require.Contains(t, err.Error(), ".gitignore", "expected .gitignore error, got: %v", err)
}

func TestRunMigrate_StringProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("cdktf.out\ngenerated\n"), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.NoError(t, err, "unexpected error")

	// cdktf.json deleted
	_, err = os.Stat(filepath.Join(dir, "cdktf.json"))
	require.True(t, os.IsNotExist(err), "cdktf.json should be deleted")

	// package.json deleted
	_, err = os.Stat(filepath.Join(dir, "package.json"))
	require.True(t, os.IsNotExist(err), "package.json should be deleted")

	// aws/generate.go created
	genData, err := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
	require.NoError(t, err, "aws/generate.go not created")
	genContent := string(genData)
	require.Contains(t, genContent, "//go:generate tforge generate -provider hashicorp/aws -version 5.0 -package aws -output .", "wrong generate directive: %s", genContent)
	require.Contains(t, genContent, "package aws", "wrong package name: %s", genContent)

	// .gitignore updated (only appends, does not remove existing entries)
	gitignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	gi := string(gitignore)
	require.Contains(t, gi, "cdktf.out", ".gitignore should still contain original cdktf.out entry")
	require.Contains(t, gi, "*_gen.go", ".gitignore should contain *_gen.go")
}

func TestRunMigrate_ObjectProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": [{"name":"aws","source":"hashicorp/aws","version":"~> 3.22"}]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.NoError(t, err, "unexpected error")

	genData, _ := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
	require.Contains(t, string(genData), "-provider hashicorp/aws -version 3.22", "wrong generate directive: %s", string(genData))
}

func TestRunMigrate_ProviderWithoutVersion(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/random"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.NoError(t, err, "unexpected error")

	genData, _ := os.ReadFile(filepath.Join(dir, "random", "generate.go"))
	require.Contains(t, string(genData), "-version latest", "expected 'latest' version: %s", string(genData))
	require.Contains(t, string(genData), "package random", "expected package random: %s", string(genData))
}

func TestRunMigrate_MultipleProviders(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["hashicorp/aws@~> 5.0", "datadog/datadog@~> 3.39"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.NoError(t, err, "unexpected error")

	// Both provider packages created
	_, err = os.Stat(filepath.Join(dir, "aws", "generate.go"))
	require.NoError(t, err, "aws/generate.go not created")
	_, err = os.Stat(filepath.Join(dir, "datadog", "generate.go"))
	require.NoError(t, err, "datadog/generate.go not created")

	awsGen, _ := os.ReadFile(filepath.Join(dir, "aws", "generate.go"))
	require.Contains(t, string(awsGen), "package aws", "aws package should be named aws")
	ddGen, _ := os.ReadFile(filepath.Join(dir, "datadog", "generate.go"))
	require.Contains(t, string(ddGen), "package datadog", "datadog package should be named datadog")
}

func TestRunMigrate_ThirdPartyProvider(t *testing.T) {
	dir := t.TempDir()
	cdktf := `{"terraformProviders": ["kreuzwerker/docker@~> 2.11"]}`
	os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(cdktf), 0644)

	var stdout, stderr bytes.Buffer
	err := runMigrate([]string{"-dir", dir}, &stdout, &stderr)
	require.NoError(t, err, "unexpected error")

	genData, _ := os.ReadFile(filepath.Join(dir, "docker", "generate.go"))
	content := string(genData)
	require.Contains(t, content, "-provider kreuzwerker/docker", "expected kreuzwerker/docker provider: %s", content)
	require.Contains(t, content, "package docker", "expected package docker: %s", content)
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
			require.NoError(t, err, "unexpected error")
			require.Equal(t, tt.source, p.Source, "source = %q, want %q", p.Source, tt.source)
			require.Equal(t, tt.name, p.Name, "name = %q, want %q", p.Name, tt.name)
			require.Equal(t, tt.version, p.Version, "version = %q, want %q", p.Version, tt.version)
		})
	}
}
