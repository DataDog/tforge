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

	"github.com/dave/jennifer/jen"

	"github.com/DataDog/tforge/provider"
)

func TestNeedsPointer(t *testing.T) {
	tests := []struct {
		name   string
		attr   *provider.Attribute
		goType string
		want   bool
	}{
		{"optional float64", &provider.Attribute{Optional: true}, "float64", true},
		{"optional bool", &provider.Attribute{Optional: true}, "bool", true},
		{"optional string", &provider.Attribute{Optional: true}, "string", false},
		{"optional slice", &provider.Attribute{Optional: true}, "[]string", false},
		{"required float64", &provider.Attribute{Required: true}, "float64", false},
		{"required bool", &provider.Attribute{Required: true}, "bool", false},
		{"computed-only float64", &provider.Attribute{Computed: true}, "float64", false},
		{"computed-only bool", &provider.Attribute{Computed: true}, "bool", false},
		{"optional+computed float64", &provider.Attribute{Optional: true, Computed: true}, "float64", true},
		{"optional+computed bool", &provider.Attribute{Optional: true, Computed: true}, "bool", true},
		{"nil attr", nil, "float64", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsPointer(tt.attr, tt.goType)
			if got != tt.want {
				t.Errorf("needsPointer(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// compileGeneratedCode creates a temporary Go module in the given directory
// and runs "go build ./..." to verify the generated code compiles.
func compileGeneratedCode(t *testing.T, dir, pkgName string) {
	t.Helper()

	// Write a go.mod that can resolve github.com/DataDog/tforge/config (used by generated constructors)
	tforgeRoot, _ := filepath.Abs(filepath.Join("..", "."))
	goMod := fmt.Sprintf("module testcompile\n\ngo 1.25.9\n\nrequire github.com/DataDog/tforge v0.0.0\nreplace github.com/DataDog/tforge => %s\n", tforgeRoot)
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

	ps := &provider.ProviderSchema{

		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{},
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

	ps := &provider.ProviderSchema{

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
	// Direct unit tests for the attributeType -> goTypeName pipeline.
	// Each test builds a minimal Attribute and verifies the exact Go type string.
	tests := []struct {
		name     string
		attr     *provider.Attribute
		wantType string
		wantGo   string
	}{
		{
			name:     "string",
			attr:     &provider.Attribute{Type: json.RawMessage(`"string"`)},
			wantType: "string",
			wantGo:   "string",
		},
		{
			name:     "number",
			attr:     &provider.Attribute{Type: json.RawMessage(`"number"`)},
			wantType: "number",
			wantGo:   "float64",
		},
		{
			name:     "bool",
			attr:     &provider.Attribute{Type: json.RawMessage(`"bool"`)},
			wantType: "bool",
			wantGo:   "bool",
		},
		{
			name:     "dynamic",
			attr:     &provider.Attribute{Type: json.RawMessage(`"dynamic"`)},
			wantType: "dynamic",
			wantGo:   "any",
		},
		{
			name:     "list of string",
			attr:     &provider.Attribute{Type: json.RawMessage(`["list","string"]`)},
			wantType: "list(string)",
			wantGo:   "[]string",
		},
		{
			name:     "map of number",
			attr:     &provider.Attribute{Type: json.RawMessage(`["map","number"]`)},
			wantType: "map(number)",
			wantGo:   "map[string]float64",
		},
		{
			name:     "set of bool",
			attr:     &provider.Attribute{Type: json.RawMessage(`["set","bool"]`)},
			wantType: "set(bool)",
			wantGo:   "[]bool",
		},
		{
			name:     "nested list of list",
			attr:     &provider.Attribute{Type: json.RawMessage(`["list",["list","string"]]`)},
			wantType: "list(list(string))",
			wantGo:   "[][]string",
		},
		{
			name:     "nil attribute",
			attr:     nil,
			wantType: "dynamic",
			wantGo:   "any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType := attributeType(tt.attr)
			if gotType != tt.wantType {
				t.Errorf("attributeType() = %q, want %q", gotType, tt.wantType)
			}
			gotGo := goTypeName(gotType)
			if gotGo != tt.wantGo {
				t.Errorf("goTypeName(%q) = %q, want %q", gotType, gotGo, tt.wantGo)
			}
		})
	}
}

func TestApplyGoTypePointer(t *testing.T) {
	// Verify that applyGoType handles pointer types correctly by rendering
	// the jen statement to a string and checking the output.
	tests := []struct {
		goType string
		want   string
	}{
		{"*float64", "*float64"},
		{"*bool", "*bool"},
		{"float64", "float64"},
		{"bool", "bool"},
		{"string", "string"},
	}
	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			stmt := applyGoType(jen.Null(), tt.goType)
			got := fmt.Sprintf("%#v", stmt)
			if got != tt.want {
				t.Errorf("applyGoType(%q) rendered %q, want %q", tt.goType, got, tt.want)
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

	ps := &provider.ProviderSchema{

		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{
				Attributes: map[string]*provider.Attribute{
					"api_key": {Type: json.RawMessage(`"string"`), Required: true},
				},
				BlockTypes: map[string]*provider.NestedBlock{
					"settings": {
						NestingMode: provider.NestingModeSingle,
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
							NestingMode: provider.NestingModeList,
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

	ps := &provider.ProviderSchema{

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
							NestingMode: provider.NestingModeSet,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"key": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
						"metadata": {
							NestingMode: provider.NestingModeMap,
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

	ps := &provider.ProviderSchema{

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

	ps := &provider.ProviderSchema{

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
						NestingMode: provider.NestingModeSingle,
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
							NestingMode: provider.NestingModeList,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"size": {Type: json.RawMessage(`"number"`), Required: true},
									"type": {Type: json.RawMessage(`"string"`), Optional: true},
								},
							},
						},
						"network": {
							NestingMode: provider.NestingModeSet,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"cidr": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
						"metadata": {
							NestingMode: provider.NestingModeMap,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"value": {Type: json.RawMessage(`"string"`), Optional: true},
								},
							},
						},
						"primary_nic": {
							NestingMode: provider.NestingModeSingle,
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
							NestingMode: provider.NestingModeList,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"region": {Type: json.RawMessage(`"string"`), Required: true},
								},
								BlockTypes: map[string]*provider.NestedBlock{
									"endpoint": {
										NestingMode: provider.NestingModeSingle,
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
							NestingMode: provider.NestingModeSet,
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

// TestGenerateNestedTypeAttributes verifies that attributes with NestedType
// (inline nested schemas, as opposed to BlockTypes) generate correct Go code
// for all nesting modes: single, list, set, map, and empty attributes.
// This is the primary guard against the generator mishandling NestedType
// attributes (e.g., emitting flat map[string]any instead of typed structs).
func TestGenerateNestedTypeAttributes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-nestedtype-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	// Use a schema with all NestedType nesting modes.
	ps := &provider.ProviderSchema{

		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{},
		},
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_nested_types": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name": {Type: json.RawMessage(`"string"`), Required: true},
						"single_nested": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"inner_name":  {Type: json.RawMessage(`"string"`), Required: true},
									"inner_count": {Type: json.RawMessage(`"number"`), Optional: true},
								},
								NestingMode: provider.NestingModeSingle,
							},
							Optional: true,
						},
						"list_nested": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"value": {Type: json.RawMessage(`"string"`), Required: true},
								},
								NestingMode: provider.NestingModeList,
							},
							Optional: true,
						},
						"set_nested": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"key":   {Type: json.RawMessage(`"string"`), Required: true},
									"value": {Type: json.RawMessage(`"string"`), Optional: true},
								},
								NestingMode: provider.NestingModeSet,
							},
							Optional: true,
						},
						"map_nested": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"setting": {Type: json.RawMessage(`"string"`), Required: true},
								},
								NestingMode: provider.NestingModeMap,
							},
							Optional: true,
						},
						"empty_nested": {
							NestedType: &provider.NestedType{
								Attributes:  map[string]*provider.Attribute{},
								NestingMode: provider.NestingModeSingle,
							},
							Optional: true,
						},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify resources_gen.go exists
	resourcePath := filepath.Join(tmpDir, "resources_gen.go")
	data, err := os.ReadFile(resourcePath)
	if err != nil {
		t.Fatalf("resources_gen.go not found: %v", err)
	}

	content := string(data)

	// NestedType attributes with nesting mode "single" should produce
	// a Go type mapped to map[string]any (via goTypeName("object")).
	// list → []map[string]any, set → []map[string]any, map → map[string]map[string]any.
	// Verify the struct has the expected fields.
	for _, want := range []string{
		"NestedTypes",   // struct name (test_nested_types → NestedTypes with provider prefix stripped)
		"SingleNested",  // field from single_nested attribute
		"ListNested",    // field from list_nested attribute
		"SetNested",     // field from set_nested attribute
		"MapNested",     // field from map_nested attribute
		"EmptyNested",   // field from empty_nested attribute
		"json:\"name\"", // required field has no omitempty
	} {
		if !strings.Contains(content, want) {
			t.Errorf("resources_gen.go does not contain %q", want)
		}
	}

	// Optional nested type attributes should have omitempty
	for _, want := range []string{
		"json:\"single_nested,omitempty\"",
		"json:\"list_nested,omitempty\"",
		"json:\"set_nested,omitempty\"",
		"json:\"map_nested,omitempty\"",
		"json:\"empty_nested,omitempty\"",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("resources_gen.go does not contain %q", want)
		}
	}

	// The critical check: verify the generated code actually compiles.
	// This catches type errors like generating map[string]any when a
	// typed struct was expected.
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestDetectProviderPrefix(t *testing.T) {
	tests := []struct {
		name       string
		resources  map[string]*provider.SchemaBlock
		dataSrcs   map[string]*provider.SchemaBlock
		wantPrefix string
	}{
		{
			name:       "resources with underscore",
			resources:  map[string]*provider.SchemaBlock{"datadog_monitor": {}, "datadog_slo": {}},
			wantPrefix: "Datadog",
		},
		{
			name:       "data sources only",
			dataSrcs:   map[string]*provider.SchemaBlock{"aws_ami": {}},
			wantPrefix: "Aws",
		},
		{
			name:       "resources preferred over data sources",
			resources:  map[string]*provider.SchemaBlock{"datadog_monitor": {}},
			dataSrcs:   map[string]*provider.SchemaBlock{"aws_ami": {}},
			wantPrefix: "Datadog",
		},
		{
			name:       "no underscore in resource name",
			resources:  map[string]*provider.SchemaBlock{"singleword": {}},
			wantPrefix: "",
		},
		{
			name:       "no resources or data sources",
			wantPrefix: "",
		},
		{
			name:       "falls through to data sources when resource has no underscore",
			resources:  map[string]*provider.SchemaBlock{"nounderscore": {}},
			dataSrcs:   map[string]*provider.SchemaBlock{"aws_ami": {}},
			wantPrefix: "Aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New(Config{PackageName: "test", OutputDir: t.TempDir()})
			g.schema = &provider.ProviderSchema{
				ResourceSchemas:   tt.resources,
				DataSourceSchemas: tt.dataSrcs,
			}
			got := g.detectProviderPrefix()
			if got != tt.wantPrefix {
				t.Errorf("detectProviderPrefix() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

func TestShortName(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		structName string
		want       string
	}{
		{
			name:       "strips matching prefix",
			prefix:     "Datadog",
			structName: "DatadogMonitor",
			want:       "Monitor",
		},
		{
			name:       "no prefix set",
			prefix:     "",
			structName: "DatadogMonitor",
			want:       "DatadogMonitor",
		},
		{
			name:       "prefix does not match",
			prefix:     "Aws",
			structName: "DatadogMonitor",
			want:       "DatadogMonitor",
		},
		{
			name:       "stripping prefix leaves empty string",
			prefix:     "Datadog",
			structName: "Datadog",
			want:       "Datadog",
		},
		{
			name:       "initialism prefix",
			prefix:     "AWS",
			structName: "AWSInstance",
			want:       "Instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New(Config{PackageName: "test", OutputDir: t.TempDir()})
			g.providerPrefix = tt.prefix
			got := g.shortName(tt.structName)
			if got != tt.want {
				t.Errorf("shortName(%q) = %q, want %q", tt.structName, got, tt.want)
			}
		})
	}
}

// TestConstructorRoundTrip generates code, writes a test harness that invokes
// the generated constructors, and runs it in a subprocess. This verifies that
// the marshal/unmarshal round-trip inside the constructor preserves field values
// correctly (required strings, optional bools set to false, nested blocks, data
// source constructors, and omitted optional fields).
func TestConstructorRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-roundtrip-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a sub-package for generated code
	genDir := filepath.Join(tmpDir, "gen")
	if err := os.MkdirAll(genDir, 0755); err != nil {
		t.Fatal(err)
	}

	g := New(Config{
		PackageName: "gen",
		OutputDir:   genDir,
	})

	ps := &provider.ProviderSchema{

		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{},
		},
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_widget": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name":    {Type: json.RawMessage(`"string"`), Required: true},
						"enabled": {Type: json.RawMessage(`"bool"`), Optional: true},
						"count":   {Type: json.RawMessage(`"number"`), Optional: true},
						"tags":    {Type: json.RawMessage(`["list","string"]`), Optional: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"setting": {
							NestingMode: provider.NestingModeList,
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
			"test_lookup": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"filter": {Type: json.RawMessage(`"string"`), Optional: true},
						"id":     {Type: json.RawMessage(`"string"`), Computed: true},
					},
				},
			},
		},
	}

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Write go.mod at tmpDir level
	tforgeRoot, _ := filepath.Abs(filepath.Join("..", "."))
	goMod := fmt.Sprintf("module roundtrip_test\n\ngo 1.25.9\n\nrequire github.com/DataDog/tforge v0.0.0\nreplace github.com/DataDog/tforge => %s\n", tforgeRoot)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Write test harness that exercises the generated constructors
	testCode := `package gen

import (
	"encoding/json"
	"testing"

	"github.com/DataDog/tforge/config"
)

func TestResourceConstructorRoundTrip(t *testing.T) {
	stack := config.NewStack("rt")

	// Case 1: Required string field
	err := NewWidget(stack, "basic", Widget_SetName("hello"))
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	if len(stack.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(stack.Resources))
	}
	r := stack.Resources[0]
	if r.Type != "test_widget" {
		t.Errorf("type = %q, want test_widget", r.Type)
	}
	if r.Name != "basic" {
		t.Errorf("name = %q, want basic", r.Name)
	}
	if r.Config["name"] != "hello" {
		t.Errorf("config[name] = %v, want hello", r.Config["name"])
	}

	// Case 2: Optional bool set to false must appear in config (not omitted).
	// With pointer types, SetEnabled(false) produces &false which is NOT omitted
	// by json omitempty — this is the correct Terraform behavior where
	// "enabled = false" is different from not setting "enabled" at all.
	stack2 := config.NewStack("rt2")
	err = NewWidget(stack2, "booltest",
		Widget_SetName("x"),
		Widget_SetEnabled(false),
	)
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r2 := stack2.Resources[0]
	if _, exists := r2.Config["enabled"]; !exists {
		t.Error("config[enabled] should be present when explicitly set to false")
	} else if r2.Config["enabled"] != false {
		t.Errorf("config[enabled] = %v, want false", r2.Config["enabled"])
	}

	// Case 2b: Optional float64 set to 0 must appear in config (not omitted).
	// Same zero-value ambiguity as bool: Terraform "count = 0" differs from unset.
	stack2b := config.NewStack("rt2b")
	err = NewWidget(stack2b, "zerocount",
		Widget_SetName("x"),
		Widget_SetCount(0),
	)
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r2b := stack2b.Resources[0]
	if _, exists := r2b.Config["count"]; !exists {
		t.Error("config[count] should be present when explicitly set to 0")
	} else if r2b.Config["count"] != float64(0) {
		t.Errorf("config[count] = %v, want 0", r2b.Config["count"])
	}

	// Case 3: Optional bool set to true must appear
	stack3 := config.NewStack("rt3")
	err = NewWidget(stack3, "truebool",
		Widget_SetName("y"),
		Widget_SetEnabled(true),
	)
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r3 := stack3.Resources[0]
	if r3.Config["enabled"] != true {
		t.Errorf("config[enabled] = %v, want true", r3.Config["enabled"])
	}

	// Case 4: Nested block round-trip
	stack4 := config.NewStack("rt4")
	err = NewWidget(stack4, "nested",
		Widget_SetName("z"),
		Widget_SetSetting(&WidgetSetting{Key: "k1", Value: "v1"}),
	)
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r4 := stack4.Resources[0]
	settingRaw, ok := r4.Config["setting"]
	if !ok {
		t.Fatal("config[setting] missing")
	}
	// The constructor marshals/unmarshals to map[string]any, so nested blocks
	// come through as []any containing map[string]any.
	settingSlice, ok := settingRaw.([]any)
	if !ok {
		t.Fatalf("config[setting] is %T, want []any", settingRaw)
	}
	if len(settingSlice) != 1 {
		t.Fatalf("len(setting) = %d, want 1", len(settingSlice))
	}
	settingMap := settingSlice[0].(map[string]any)
	if settingMap["key"] != "k1" {
		t.Errorf("setting[0].key = %v, want k1", settingMap["key"])
	}
	if settingMap["value"] != "v1" {
		t.Errorf("setting[0].value = %v, want v1", settingMap["value"])
	}

	// Case 5: Empty optional fields are absent
	stack5 := config.NewStack("rt5")
	err = NewWidget(stack5, "minimal", Widget_SetName("only-name"))
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r5 := stack5.Resources[0]
	for _, key := range []string{"tags", "count", "enabled", "setting"} {
		if _, exists := r5.Config[key]; exists {
			t.Errorf("config[%s] should be absent for unset optional, got %v", key, r5.Config[key])
		}
	}

	// Case 6: Verify the config can be rendered to valid JSON
	data, err := json.Marshal(r5.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if !json.Valid(data) {
		t.Error("config is not valid JSON")
	}
}

func TestDataSourceConstructorRoundTrip(t *testing.T) {
	stack := config.NewStack("ds")

	err := NewLookupData(stack, "myds", LookupData_SetFilter("active"))
	if err != nil {
		t.Fatalf("NewLookupData: %v", err)
	}
	if len(stack.DataSources) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(stack.DataSources))
	}
	ds := stack.DataSources[0]
	if ds.Type != "test_lookup" {
		t.Errorf("type = %q, want test_lookup", ds.Type)
	}
	if ds.Name != "myds" {
		t.Errorf("name = %q, want myds", ds.Name)
	}
	if ds.Config["filter"] != "active" {
		t.Errorf("config[filter] = %v, want active", ds.Config["filter"])
	}
}

func TestFromBridgeRoundTrip(t *testing.T) {
	stack := config.NewStack("bridge")

	cfg := Widget{
		Name:    "direct",
		Enabled: Ptr(true),
		Tags:    []string{"a", "b"},
	}
	err := NewWidget(stack, "bridged", FromWidget(cfg))
	if err != nil {
		t.Fatalf("NewWidget with FromWidget: %v", err)
	}
	r := stack.Resources[0]
	if r.Config["name"] != "direct" {
		t.Errorf("config[name] = %v, want direct", r.Config["name"])
	}
	if r.Config["enabled"] != true {
		t.Errorf("config[enabled] = %v, want true", r.Config["enabled"])
	}
	tagsRaw := r.Config["tags"].([]any)
	if len(tagsRaw) != 2 || tagsRaw[0] != "a" || tagsRaw[1] != "b" {
		t.Errorf("config[tags] = %v, want [a b]", tagsRaw)
	}
}
`
	if err := os.WriteFile(filepath.Join(genDir, "roundtrip_test.go"), []byte(testCode), 0644); err != nil {
		t.Fatalf("writing roundtrip_test.go: %v", err)
	}

	// Run go test in the temp directory
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "./gen/...")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("constructor round-trip tests failed:\n%s\n%v", out, err)
	}
	t.Logf("constructor round-trip output:\n%s", out)
}

// TestGenerateNestedTypeFullFixture verifies that the full test fixture
// (newTestProviderSchema) with NestedType attributes generates compilable code.
// This is a broader smoke test using the same fixture as schema_test.go.
func TestGenerateNestedTypeFullFixture(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-fullfixture-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := newTestProviderSchema()

	if err := g.Generate(ps); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all expected files
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("%s not found: %v", name, err)
		}
	}

	// Verify generated code compiles (including all NestedType attributes)
	compileGeneratedCode(t, tmpDir, "testpkg")
}
