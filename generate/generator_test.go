package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/require"

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
			require.Equal(t, tt.want, got, "needsPointer(%q) = %v, want %v", tt.name, got, tt.want)
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644), "writing go.mod")

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "generated code does not compile:\n%s", out)
}

func TestNewDefaults(t *testing.T) {
	g := New(Config{})
	require.Equal(t, "generated", g.config.PackageName, "default PackageName = %q, want %q", g.config.PackageName, "generated")
	require.Equal(t, "./generated", g.config.OutputDir, "default OutputDir = %q, want %q", g.config.OutputDir, "./generated")
}

func TestNewCustomValues(t *testing.T) {
	g := New(Config{PackageName: "foo", OutputDir: "/tmp"})
	require.Equal(t, "foo", g.config.PackageName, "PackageName = %q, want %q", g.config.PackageName, "foo")
	require.Equal(t, "/tmp", g.config.OutputDir, "OutputDir = %q, want %q", g.config.OutputDir, "/tmp")
}

func TestGenerateNilSchema(t *testing.T) {
	g := New(Config{})
	err := g.Generate(nil)
	require.Error(t, err)
	require.Equal(t, "nil provider schema", err.Error(), "error = %q, want %q", err.Error(), "nil provider schema")
}

func TestGenerateEmptySchema(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-test-*")
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	// provider_gen.go should exist
	_, err = os.Stat(filepath.Join(tmpDir, "provider_gen.go"))
	require.NoError(t, err, "provider_gen.go not found")

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestGenerateWithResources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-test-*")
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	// Verify generated files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go"} {
		path := filepath.Join(tmpDir, name)
		info, err := os.Stat(path)
		require.NoError(t, err, "%s not found", name)
		require.NotEqual(t, int64(0), info.Size(), "%s is empty", name)
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
	require.Equal(t, "name", got, "jsonTag(name, false) = %q, want %q", got, "name")

	got = g.jsonTag("name", true)
	require.Equal(t, "name,omitempty", got, "jsonTag(name, true) = %q, want %q", got, "name,omitempty")
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
			require.Equal(t, tt.wantType, gotType, "attributeType() = %q, want %q", gotType, tt.wantType)
			gotGo := goTypeName(gotType)
			require.Equal(t, tt.wantGo, gotGo, "goTypeName(%q) = %q, want %q", gotType, gotGo, tt.wantGo)
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
			require.Equal(t, tt.want, got, "applyGoType(%q) rendered %q, want %q", tt.goType, got, tt.want)
		})
	}
}

func TestGenerateWithNestedBlocksAndDataSources(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-nested-*")
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	// Verify all generated files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		path := filepath.Join(tmpDir, name)
		info, err := os.Stat(path)
		require.NoError(t, err, "%s not found", name)
		require.NotEqual(t, int64(0), info.Size(), "%s is empty", name)
	}

	// Verify provider_gen.go contains nested block type
	providerData, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
	require.NoError(t, err, "reading provider_gen.go")
	providerContent := string(providerData)
	for _, want := range []string{"ProviderConfig", "Settings", "ProviderConfigSettings"} {
		require.Contains(t, providerContent, want, "provider_gen.go does not contain %q", want)
	}

	// Verify resources_gen.go contains nested block type
	resourceData, err := os.ReadFile(filepath.Join(tmpDir, "resources_gen.go"))
	require.NoError(t, err, "reading resources_gen.go")
	resourceContent := string(resourceData)
	for _, want := range []string{
		"Thing", "Config", "ThingConfig",
		"ThingConfigOption", "NewThingConfig", "ThingConfig_SetKey",
	} {
		require.Contains(t, resourceContent, want, "resources_gen.go does not contain %q", want)
	}

	// Verify data_sources_gen.go content
	dsData, err := os.ReadFile(filepath.Join(tmpDir, "data_sources_gen.go"))
	require.NoError(t, err, "reading data_sources_gen.go")
	dsContent := string(dsData)
	for _, want := range []string{"DataData", "Filter", "ID"} {
		require.Contains(t, dsContent, want, "data_sources_gen.go does not contain %q", want)
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

func TestGenerateWithSetAndMapNesting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-nesting-*")
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	resourceData, err := os.ReadFile(filepath.Join(tmpDir, "resources_gen.go"))
	require.NoError(t, err, "reading resources_gen.go")
	resourceContent := string(resourceData)

	// Check that set and map nesting modes produce correct types
	for _, want := range []string{"Multi", "MultiTags", "MultiMetadata"} {
		require.Contains(t, resourceContent, want, "resources_gen.go does not contain %q", want)
	}

	// Verify generated code compiles
	compileGeneratedCode(t, tmpDir, "testpkg")
}

// TestGenerateMaxItemsOneCollapse verifies the cdktf-compatible collapse of
// `MaxItems = 1` list/set nested blocks into single-pointer fields and setters.
// A sibling nested block without MaxItems must remain a slice/variadic so the
// rule only applies where the schema explicitly requests it.
func TestGenerateMaxItemsOneCollapse(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-maxitems-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	g := New(Config{PackageName: "testpkg", OutputDir: tmpDir})

	ps := &provider.ProviderSchema{
		ConfigSchema: &provider.SchemaBlock{Block: &provider.BlockType{}},
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_monitor": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name": {Type: json.RawMessage(`"string"`), Required: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"monitor_thresholds": {
							NestingMode: provider.NestingModeList,
							MaxItems:    1,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"critical": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
						"singleton_set": {
							NestingMode: provider.NestingModeSet,
							MaxItems:    1,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"key": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
						"tag": {
							NestingMode: provider.NestingModeSet,
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"key": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, g.Generate(ps))

	data, err := os.ReadFile(filepath.Join(tmpDir, "resources_gen.go"))
	require.NoError(t, err)
	content := string(data)

	// gofmt aligns struct field columns with arbitrary whitespace, so match shapes via regex.
	require.Regexp(t, `MonitorThresholds\s+\*MonitorMonitorThresholds\s`, content,
		"MaxItems=1 list block should be a single pointer field")
	require.Regexp(t, `SingletonSet\s+\*MonitorSingletonSet\s`, content,
		"MaxItems=1 set block should be a single pointer field")
	require.Regexp(t, `Tag\s+\[\]\*MonitorTag\s`, content,
		"set block without MaxItems should remain a slice field")

	require.Contains(t, content, "func Monitor_SetMonitorThresholds(v *MonitorMonitorThresholds)",
		"MaxItems=1 setter should take a single pointer, not variadic")
	require.Contains(t, content, "func Monitor_SetSingletonSet(v *MonitorSingletonSet)",
		"MaxItems=1 setter should take a single pointer, not variadic")
	require.Contains(t, content, "func Monitor_SetTag(v ...*MonitorTag)",
		"plain set setter should remain variadic")

	compileGeneratedCode(t, tmpDir, "testpkg")
}

// TestGenerateProviderMetadata verifies that when the generator is given the
// provider source and version, it emits a `Provider` variable of type
// `config.ProviderMetadata` and a NewProvider helper that reads from it.
// Without the metadata in the Config, neither should be emitted
// (backwards-compatible).
func TestGenerateProviderMetadata(t *testing.T) {
	schema := &provider.ProviderSchema{
		ConfigSchema: &provider.SchemaBlock{Block: &provider.BlockType{}},
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"datadog_thing": {Block: &provider.BlockType{}},
		},
	}

	t.Run("with metadata", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "tforge-gen-providermeta-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		g := New(Config{
			PackageName:     "testpkg",
			OutputDir:       tmpDir,
			ProviderSource:  "DataDog/datadog",
			ProviderVersion: "3.39.0",
		})
		require.NoError(t, g.Generate(schema))

		data, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
		require.NoError(t, err)
		content := string(data)

		require.Contains(t, content, "var Provider = config.ProviderMetadata{",
			"Provider should be declared as a config.ProviderMetadata value")
		require.Contains(t, content, `Name:    "datadog"`,
			"local provider name should come from the resource type prefix")
		require.Contains(t, content, `Source:  "DataDog/datadog"`)
		require.Contains(t, content, `Version: "3.39.0"`)
		require.Contains(t, content, "func NewProvider(stack *config.Stack, opts ...config.ProviderOption)",
			"NewProvider helper signature should be emitted")
		require.Contains(t, content, "stack.AddProvider(p)",
			"NewProvider should call AddProvider")
		require.Contains(t, content, "Provider.Name",
			"NewProvider should read Name from the Provider struct")

		compileGeneratedCode(t, tmpDir, "testpkg")
	})

	t.Run("without metadata", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "tforge-gen-no-providermeta-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		g := New(Config{PackageName: "testpkg", OutputDir: tmpDir})
		require.NoError(t, g.Generate(schema))

		data, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
		require.NoError(t, err)
		content := string(data)

		require.NotContains(t, content, "ProviderMetadata",
			"metadata var must not leak when ProviderSource is unset")
		require.NotContains(t, content, "NewProvider",
			"NewProvider helper must not leak when the metadata is missing")

		compileGeneratedCode(t, tmpDir, "testpkg")
	})

	t.Run("local name from prefix (google-beta source)", func(t *testing.T) {
		// google-beta provider's source name (`google-beta`) differs from the
		// local name resources bind to (`google`). The generator must use the
		// resource type prefix, not the source suffix.
		googleSchema := &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{Block: &provider.BlockType{}},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"google_compute_instance": {Block: &provider.BlockType{}},
			},
		}
		tmpDir, err := os.MkdirTemp("", "tforge-gen-googlebeta-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		g := New(Config{
			PackageName:     "testpkg",
			OutputDir:       tmpDir,
			ProviderSource:  "hashicorp/google-beta",
			ProviderVersion: "5.0.0",
		})
		require.NoError(t, g.Generate(googleSchema))

		data, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
		require.NoError(t, err)
		content := string(data)

		require.Contains(t, content, `Name:    "google"`,
			"local name must come from the resource type prefix, not the source suffix")
		require.Contains(t, content, `Source:  "hashicorp/google-beta"`,
			"Source should preserve the full registry address")
	})

	t.Run("collision skipped (resource shadows Provider)", func(t *testing.T) {
		// A `<prefix>_provider` resource short-names to `Provider`, which would
		// conflict with the metadata var. The generator must skip emission.
		schemaWithCollision := &provider.ProviderSchema{
			ConfigSchema: &provider.SchemaBlock{Block: &provider.BlockType{}},
			ResourceSchemas: map[string]*provider.SchemaBlock{
				"datadog_thing":    {Block: &provider.BlockType{}},
				"datadog_provider": {Block: &provider.BlockType{}}, // -> type Provider
			},
		}
		tmpDir, err := os.MkdirTemp("", "tforge-gen-collision-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		g := New(Config{
			PackageName:     "testpkg",
			OutputDir:       tmpDir,
			ProviderSource:  "DataDog/datadog",
			ProviderVersion: "3.39.0",
		})
		require.NoError(t, g.Generate(schemaWithCollision))

		data, err := os.ReadFile(filepath.Join(tmpDir, "provider_gen.go"))
		require.NoError(t, err)
		content := string(data)

		require.NotContains(t, content, "var Provider = config.ProviderMetadata{",
			"metadata var must be skipped when a resource shadows `Provider`")
		require.NotContains(t, content, "func NewProvider(stack",
			"NewProvider must also be skipped to avoid clashing with the resource constructor")

		compileGeneratedCode(t, tmpDir, "testpkg")
	})
}

func TestGenerateWithComments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-comments-*")
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

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
		require.NoError(t, err, "%s not found", tc.file)
		content := string(data)
		for _, exp := range tc.expected {
			require.Contains(t, content, exp, "%s: expected comment %q not found in generated output", tc.file, exp)
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
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	// Verify all expected files exist
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		_, err := os.Stat(filepath.Join(tmpDir, name))
		require.NoError(t, err, "%s not found", name)
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
	require.NoError(t, err)
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

	require.NoError(t, g.Generate(ps), "Generate failed")

	// Verify resources_gen.go exists
	resourcePath := filepath.Join(tmpDir, "resources_gen.go")
	data, err := os.ReadFile(resourcePath)
	require.NoError(t, err, "resources_gen.go not found")

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
		require.Contains(t, content, want, "resources_gen.go does not contain %q", want)
	}

	// Optional nested type attributes should have omitempty
	for _, want := range []string{
		"json:\"single_nested,omitempty\"",
		"json:\"list_nested,omitempty\"",
		"json:\"set_nested,omitempty\"",
		"json:\"map_nested,omitempty\"",
		"json:\"empty_nested,omitempty\"",
	} {
		require.Contains(t, content, want, "resources_gen.go does not contain %q", want)
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
			require.Equal(t, tt.wantPrefix, got, "detectProviderPrefix() = %q, want %q", got, tt.wantPrefix)
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
			require.Equal(t, tt.want, got, "shortName(%q) = %q, want %q", tt.structName, got, tt.want)
		})
	}
}

// Note: the marshal/unmarshal round-trip coverage that used to live here as
// an embedded subprocess test now lives at internal/handletest/gen as a
// regular Go test package (see internal/handletest/gen/roundtrip_test.go).
// It runs as part of `go test ./...` like any other test.

// TestGenerateNestedTypeFullFixture verifies that the full test fixture
// (newTestProviderSchema) with NestedType attributes generates compilable code.
// This is a broader smoke test using the same fixture as schema_test.go.
func TestGenerateNestedTypeFullFixture(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tforge-gen-fullfixture-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	g := New(Config{
		PackageName: "testpkg",
		OutputDir:   tmpDir,
	})

	ps := newTestProviderSchema()

	require.NoError(t, g.Generate(ps), "Generate failed")

	// Verify all expected files
	for _, name := range []string{"provider_gen.go", "resources_gen.go", "data_sources_gen.go"} {
		_, err := os.Stat(filepath.Join(tmpDir, name))
		require.NoError(t, err, "%s not found", name)
	}

	// Verify generated code compiles (including all NestedType attributes)
	compileGeneratedCode(t, tmpDir, "testpkg")
}
