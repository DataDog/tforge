package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tforge/provider"
)

// compileGeneratedCode creates a temporary Go module in the given directory
// and runs "go build ./..." to verify the generated code compiles.
func compileGeneratedCode(t *testing.T, dir, pkgName string) {
	t.Helper()

	// Write a go.mod that can resolve tforge/config (used by generated constructors)
	tforgeRoot, _ := filepath.Abs(filepath.Join("..", "."))
	goMod := fmt.Sprintf("module testcompile\n\ngo 1.23\n\nrequire tforge v0.0.0\nreplace tforge => %s\n", tforgeRoot)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code does not compile:\n%s\n%v", out, err)
	}
}

func TestNewDefaults(t *testing.T) {
	g := New(Config{})
	if g.config.PackageName != "generated" {
		t.Errorf("default PackageName = %q, want %q", g.config.PackageName, "generated")
	}
	if g.config.OutputDir != "./generated" {
		t.Errorf("default OutputDir = %q, want %q", g.config.OutputDir, "./generated")
	}
}

func TestNewCustomValues(t *testing.T) {
	g := New(Config{PackageName: "foo", OutputDir: "/tmp"})
	if g.config.PackageName != "foo" {
		t.Errorf("PackageName = %q, want %q", g.config.PackageName, "foo")
	}
	if g.config.OutputDir != "/tmp" {
		t.Errorf("OutputDir = %q, want %q", g.config.OutputDir, "/tmp")
	}
}

func TestGenerateNilSchema(t *testing.T) {
	g := New(Config{})
	err := g.Generate(nil)
	if err == nil {
		t.Fatal("expected error for nil schema, got nil")
	}
	if err.Error() != "nil provider schema" {
		t.Errorf("error = %q, want %q", err.Error(), "nil provider schema")
	}
}

func TestGenerateNilProvider(t *testing.T) {
	g := New(Config{})
	err := g.Generate(&provider.Schema{})
	if err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestGenerateEmptySchema(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// provider_gen.go should exist
	if _, err := os.Stat(filepath.Join(tmpDir, "provider_gen.go")); err != nil {
		t.Errorf("provider_gen.go not found: %v", err)
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestGenerateWithResources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"api_key": {Type: json.RawMessage(`"string"`), Required: true},
					},
				},
			},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"test_thing": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"name": {Type: json.RawMessage(`"string"`), Required: true},
							"id":   {Type: json.RawMessage(`"string"`), Computed: true},
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify generated files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go"} {
		path := filepath.Join(tmpDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s not found: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestJsonTagBehavior(t *testing.T) {
	// jsonTag always adds omitempty for optional fields.
	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   t.TempDir(),
	})

	got := g.jsonTag("name", false)
	if got != "name" {
		t.Errorf("jsonTag(name, false) = %q, want %q", got, "name")
	}

	got = g.jsonTag("name", true)
	if got != "name,omitempty" {
		t.Errorf("jsonTag(name, true) = %q, want %q", got, "name,omitempty")
	}
}

func TestGoTypeForAttribute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-gotype-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		typeJSON string
		want     string
	}{
		{"string", `"string"`, "string"},
		{"number", `"number"`, "float64"},
		{"bool", `"bool"`, "bool"},
		{"dynamic", `"dynamic"`, "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a minimal provider.Attribute to wrap in schema.Attribute
			// and pass through goTypeForAttribute
			ps := &provider.Schema{
				Provider: &provider.ProviderSchema{
					ConfigSchema: &provider.SchemaBlock{
						Block: &provider.BlockType{
							Attributes: map[string]*provider.Attribute{
								"test_attr": {Type: json.RawMessage(tt.typeJSON), Required: true},
							},
						},
					},
				},
			}

			// Generate and check the output contains the expected Go type
			outDir := filepath.Join(tmpDir, tt.name)
			g2 := New(Config{
				PackageName: "testpkg",
				OutputDir:   outDir,
			})
			if err := g2.Generate(ps); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			data, err := os.ReadFile(filepath.Join(outDir, "provider_gen.go"))
			if err != nil {
				t.Fatalf("reading generated file: %v", err)
			}

			content := string(data)
			// The field should contain the Go type
			if tt.want == "any" {
				// jen renders "any" as "interface{}" in older Go or "any" in newer
				if !strings.Contains(content, "interface{}") && !strings.Contains(content, " any ") {
					t.Errorf("generated code does not contain type %q or interface{}", tt.want)
				}
			} else {
				if !strings.Contains(content, tt.want) {
					t.Errorf("generated code does not contain type %q", tt.want)
				}
			}
		})
	}
}

func TestGenerateWithNestedBlocksAndDataSources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-nested-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"api_key": {Type: json.RawMessage(`"string"`), Required: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"settings": {
							NestingMode: "single",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"timeout": {Type: json.RawMessage(`"number"`), Optional: true},
								},
							},
						},
					},
				},
			},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"test_thing": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"name": {Type: json.RawMessage(`"string"`), Required: true},
							"id":   {Type: json.RawMessage(`"string"`), Computed: true},
						},
						BlockTypes: map[string]*provider.NestedBlock{
							"config": {
								NestingMode: "list",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"key":   {Type: json.RawMessage(`"string"`), Required: true},
										"value": {Type: json.RawMessage(`"string"`), Optional: true},
									},
								},
							},
						},
					},
				},
			},
			DataSourceSchemas: map[string]*provider.SchemaBlock{
				"test_data": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"filter": {Type: json.RawMessage(`"string"`), Optional: true},
							"id":     {Type: json.RawMessage(`"string"`), Computed: true},
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all generated files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		path := filepath.Join(tmpDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s not found: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}

	// Verify provider_gen.go contains nested block type
	providerData, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
	if err != nil {
		t.Fatalf("reading provider_gen.go: %v", err)
	}
	providerContent := string(providerData)
	for _, want := range []string{"ProviderConfig", "Settings", "ProviderConfigSettings"} {
		if !strings.Contains(providerContent, want) {
			t.Errorf("provider_gen.go does not contain %q", want)
		}
	}

	// Verify resources_gen.go contains nested block type
	resourceData, err := os.ReadFile(filepath.Join(tmpDir, "resources_gen.go"))
	if err != nil {
		t.Fatalf("reading resources_gen.go: %v", err)
	}
	resourceContent := string(resourceData)
	for _, want := range []string{"Thing", "Config", "ThingConfig"} {
		if !strings.Contains(resourceContent, want) {
			t.Errorf("resources_gen.go does not contain %q", want)
		}
	}

	// Verify data_sources_gen.go content
	dsData, err := os.ReadFile(filepath.Join(tmpDir, "data_sources_gen.go"))
	if err != nil {
		t.Fatalf("reading data_sources_gen.go: %v", err)
	}
	dsContent := string(dsData)
	for _, want := range []string{"DataData", "Filter", "ID"} {
		if !strings.Contains(dsContent, want) {
			t.Errorf("data_sources_gen.go does not contain %q", want)
		}
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestGenerateWithSetAndMapNesting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-nesting-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{},
			},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"test_multi": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"name": {Type: json.RawMessage(`"string"`), Required: true},
						},
						BlockTypes: map[string]*provider.NestedBlock{
							"tags": {
								NestingMode: "set",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"key": {Type: json.RawMessage(`"string"`), Required: true},
									},
								},
							},
							"metadata": {
								NestingMode: "map",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"value": {Type: json.RawMessage(`"string"`), Optional: true},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	resourceData, err := os.ReadFile(filepath.Join(tmpDir, "resources_gen.go"))
	if err != nil {
		t.Fatalf("reading resources_gen.go: %v", err)
	}
	resourceContent := string(resourceData)

	// Check that set and map nesting modes produce correct types
	for _, want := range []string{"Multi", "MultiTags", "MultiMetadata"} {
		if !strings.Contains(resourceContent, want) {
			t.Errorf("resources_gen.go does not contain %q", want)
		}
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestGenerateWithComments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-comments-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{
					Description: "Provider configuration for the test provider",
					Attributes: map[string]*provider.Attribute{
						"token": {Type: json.RawMessage(`"string"`), Required: true, Description: "Authentication token"},
					},
				},
			},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"test_item": {
					Block: &provider.BlockType{
						Description: "Manages a test item resource",
						Attributes: map[string]*provider.Attribute{
							"name": {Type: json.RawMessage(`"string"`), Required: true, Description: "The name of the item"},
						},
					},
				},
			},
			DataSourceSchemas: map[string]*provider.SchemaBlock{
				"test_lookup": {
					Block: &provider.BlockType{
						Description: "Reads a test lookup data source",
						Attributes: map[string]*provider.Attribute{
							"query": {Type: json.RawMessage(`"string"`), Optional: true, Description: "Search query string"},
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all three files exist and contain expected comments
	checks := []struct {
		file     string
		expected []string
	}{
		{
			file:     "provider_gen.go",
			expected: []string{"Provider configuration for the test provider"},
		},
		{
			file:     "resources_gen.go",
			expected: []string{"Manages a test item resource"},
		},
		{
			file:     "data_sources_gen.go",
			expected: []string{"Reads a test lookup data source"},
		},
	}

	for _, tc := range checks {
		data, err := os.ReadFile(filepath.Join(tmpDir, tc.file))
		if err != nil {
			t.Errorf("%s not found: %v", tc.file, err)
			continue
		}
		content := string(data)
		for _, exp := range tc.expected {
			if !strings.Contains(content, exp) {
				t.Errorf("%s: expected comment %q not found in generated output", tc.file, exp)
			}
		}
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

// TestGenerateCompilationComplex verifies that generated code with complex type
// combinations (nested lists, maps of objects, deeply nested blocks) compiles
// successfully. This is the primary guard against generating syntactically valid
// but type-incorrect Go code.
func TestGenerateCompilationComplex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-compile-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := &provider.Schema{
		Provider: &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"api_key":  {Type: json.RawMessage(`"string"`), Required: true},
						"base_url": {Type: json.RawMessage(`"string"`), Optional: true},
						"timeout":  {Type: json.RawMessage(`"number"`), Optional: true},
						"debug":    {Type: json.RawMessage(`"bool"`), Optional: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"auth": {
							NestingMode: "single",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"token": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
					},
				},
			},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"test_server": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"name":    {Type: json.RawMessage(`"string"`), Required: true},
							"id":      {Type: json.RawMessage(`"string"`), Computed: true},
							"tags":    {Type: json.RawMessage(`["list","string"]`), Optional: true},
							"labels":  {Type: json.RawMessage(`["map","string"]`), Optional: true},
							"ports":   {Type: json.RawMessage(`["set","number"]`), Optional: true},
							"dynamic": {Type: json.RawMessage(`"dynamic"`), Optional: true},
						},
						BlockTypes: map[string]*provider.NestedBlock{
							"disk": {
								NestingMode: "list",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"size": {Type: json.RawMessage(`"number"`), Required: true},
										"type": {Type: json.RawMessage(`"string"`), Optional: true},
									},
								},
							},
							"network": {
								NestingMode: "set",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"cidr": {Type: json.RawMessage(`"string"`), Required: true},
									},
								},
							},
							"metadata": {
								NestingMode: "map",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"value": {Type: json.RawMessage(`"string"`), Optional: true},
									},
								},
							},
							"primary_nic": {
								NestingMode: "single",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"mac": {Type: json.RawMessage(`"string"`), Computed: true},
									},
								},
							},
						},
					},
				},
				"test_database": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"engine":  {Type: json.RawMessage(`"string"`), Required: true},
							"version": {Type: json.RawMessage(`"string"`), Optional: true},
						},
						BlockTypes: map[string]*provider.NestedBlock{
							"replica": {
								NestingMode: "list",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"region": {Type: json.RawMessage(`"string"`), Required: true},
									},
									BlockTypes: map[string]*provider.NestedBlock{
										"endpoint": {
											NestingMode: "single",
											Block: &provider.BlockType{
												Attributes: map[string]*provider.Attribute{
													"host": {Type: json.RawMessage(`"string"`), Computed: true},
													"port": {Type: json.RawMessage(`"number"`), Computed: true},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			DataSourceSchemas: map[string]*provider.SchemaBlock{
				"test_image": {
					Block: &provider.BlockType{
						Attributes: map[string]*provider.Attribute{
							"name":   {Type: json.RawMessage(`"string"`), Optional: true},
							"id":     {Type: json.RawMessage(`"string"`), Computed: true},
							"owners": {Type: json.RawMessage(`["list","string"]`), Optional: true},
						},
						BlockTypes: map[string]*provider.NestedBlock{
							"filter": {
								NestingMode: "set",
								Block: &provider.BlockType{
									Attributes: map[string]*provider.Attribute{
										"name":   {Type: json.RawMessage(`"string"`), Required: true},
										"values": {Type: json.RawMessage(`["list","string"]`), Required: true},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all expected files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("%s not found: %v", name, err)
		}
	}

	// The critical check: verify the generated code actually compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}
