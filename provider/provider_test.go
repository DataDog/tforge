package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSourceValid(t *testing.T) {
	tests := []struct {
		input     string
		namespace string
		name      string
	}{
		{"datadog/datadog", "datadog", "datadog"},
		{"hashicorp/aws", "hashicorp", "aws"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) returned error: %v", tt.input, err)
			}
			if src.Namespace != tt.namespace {
				t.Errorf("Namespace = %q, want %q", src.Namespace, tt.namespace)
			}
			if src.Name != tt.name {
				t.Errorf("Name = %q, want %q", src.Name, tt.name)
			}
		})
	}
}

func TestParseSourceInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no slash", "invalid"},
		{"empty string", ""},
		{"empty namespace", "/name"},
		{"empty name", "ns/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if err == nil {
				t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestSourceString(t *testing.T) {
	src := Source{Namespace: "dd", Name: "dd"}
	if got := src.String(); got != "dd/dd" {
		t.Errorf("String() = %q, want %q", got, "dd/dd")
	}
}

func TestLoadSchemaFromJSONValid(t *testing.T) {
	validJSON := []byte(`{"resource_schemas":{"test_thing":{"block":{"attributes":{"id":{"type":"string","computed":true}}}}}}`)
	schema, err := LoadSchemaFromJSON(validJSON)
	if err != nil {
		t.Fatalf("LoadSchemaFromJSON returned error: %v", err)
	}
	if schema == nil {
		t.Fatal("LoadSchemaFromJSON returned nil schema")
	}
	if schema.Provider == nil {
		t.Fatal("schema.Provider is nil")
	}
	if schema.Provider.ResourceSchemas == nil {
		t.Fatal("ResourceSchemas is nil")
	}
	rs, ok := schema.Provider.ResourceSchemas["test_thing"]
	if !ok {
		t.Fatal("test_thing resource not found")
	}
	if rs.Block == nil {
		t.Fatal("test_thing block is nil")
	}
	attr, ok := rs.Block.Attributes["id"]
	if !ok {
		t.Fatal("id attribute not found")
	}
	if !attr.Computed {
		t.Error("expected id to be computed")
	}
}

func TestLoadSchemaFromJSONInvalid(t *testing.T) {
	_, err := LoadSchemaFromJSON([]byte(`{not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSaveAndLoadSchemaRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "schema.json")

	original := &Schema{
		Source:  Source{Namespace: "test", Name: "test"},
		Version: "1.0.0",
		Provider: &ProviderSchema{
			ResourceSchemas: map[string]*SchemaBlock{
				"test_resource": {
					Block: &BlockType{
						Attributes: map[string]*Attribute{
							"name": {
								Type:     []byte(`"string"`),
								Required: true,
							},
						},
						Description: "a resource",
					},
				},
			},
		},
	}

	// Marshal and write manually (SaveSchemaToFile was removed as unused)
	data, err := json.MarshalIndent(original.Provider, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadSchemaFromFile(path)
	if err != nil {
		t.Fatalf("LoadSchemaFromFile: %v", err)
	}

	if loaded.Provider == nil {
		t.Fatal("loaded Provider is nil")
	}
	rs, ok := loaded.Provider.ResourceSchemas["test_resource"]
	if !ok {
		t.Fatal("test_resource not found after roundtrip")
	}
	if rs.Block == nil {
		t.Fatal("block is nil after roundtrip")
	}
	if rs.Block.Description != "a resource" {
		t.Errorf("description = %q, want %q", rs.Block.Description, "a resource")
	}
	attr, ok := rs.Block.Attributes["name"]
	if !ok {
		t.Fatal("name attribute not found after roundtrip")
	}
	if !attr.Required {
		t.Error("expected name to be required after roundtrip")
	}
}

func TestLoadSchemaFromFileNotFound(t *testing.T) {
	_, err := LoadSchemaFromFile("/nonexistent/path/schema.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// ---------------------------------------------------------------------------
// parseSchemaOutput tests
// ---------------------------------------------------------------------------

func TestParseSchemaOutput_ExactMatch(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.opentofu.org/datadog/datadog": {
				"provider": {"block": {"attributes": {"api_key": {"type": "string"}}}},
				"resource_schemas": {
					"datadog_monitor": {"block": {"attributes": {"id": {"type": "string", "computed": true}}}}
				},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.ConfigSchema == nil {
		t.Fatal("ConfigSchema is nil")
	}
	if _, ok := ps.ResourceSchemas["datadog_monitor"]; !ok {
		t.Error("expected datadog_monitor in ResourceSchemas")
	}
}

func TestParseSchemaOutput_ContainsSrcString(t *testing.T) {
	src := Source{Namespace: "hashicorp", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_instance": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["aws_instance"]; !ok {
		t.Error("expected aws_instance in ResourceSchemas")
	}
}

func TestParseSchemaOutput_SuffixMatch(t *testing.T) {
	// Key has suffix "/name" but doesn't contain full "namespace/name"
	src := Source{Namespace: "custom", Name: "mycloud"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"localhost/other/mycloud": {
				"provider": {"block": {}},
				"resource_schemas": {"mycloud_thing": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["mycloud_thing"]; !ok {
		t.Error("expected mycloud_thing in ResourceSchemas")
	}
}

func TestParseSchemaOutput_ProviderNotFound(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.opentofu.org/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {},
				"data_source_schemas": {}
			}
		}
	}`)

	_, err := parseSchemaOutput(data, src)
	if err == nil {
		t.Fatal("expected error for provider not found, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", got)
	}
}

func TestParseSchemaOutput_EmptyProviders(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{"format_version": "1.0", "provider_schemas": {}}`)

	_, err := parseSchemaOutput(data, src)
	if err == nil {
		t.Fatal("expected error for empty provider_schemas, got nil")
	}
}

func TestParseSchemaOutput_InvalidJSON(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := parseSchemaOutput([]byte(`{not json`), src)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// LoadSchemaFromJSON edge cases
// ---------------------------------------------------------------------------

func TestLoadSchemaFromJSON_WithAllSections(t *testing.T) {
	input := `{
		"provider": {"block": {"attributes": {"key": {"type": "string"}}}},
		"resource_schemas": {"r1": {"block": {"attributes": {"id": {"type": "string", "computed": true}}}}},
		"data_source_schemas": {"d1": {"block": {"attributes": {"filter": {"type": "string", "optional": true}}}}}
	}`

	schema, err := LoadSchemaFromJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.Provider.ConfigSchema == nil {
		t.Error("ConfigSchema is nil")
	}
	if _, ok := schema.Provider.ResourceSchemas["r1"]; !ok {
		t.Error("r1 resource not found")
	}
	if _, ok := schema.Provider.DataSourceSchemas["d1"]; !ok {
		t.Error("d1 data source not found")
	}
}

func TestLoadSchemaFromJSON_EmptyObject(t *testing.T) {
	schema, err := LoadSchemaFromJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.Provider == nil {
		t.Fatal("Provider is nil")
	}
}
