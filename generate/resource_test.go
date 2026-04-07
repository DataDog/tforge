package generate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dave/jennifer/jen"

	"tforge/provider"
	"tforge/schema"
)

func TestNewResourceGenerator(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test"})
	if rg == nil {
		t.Fatal("NewResourceGenerator returned nil")
	}
	if rg.File() == nil {
		t.Fatal("File() returned nil")
	}
}

func TestGenerateResourceNil(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test"})
	err := rg.GenerateResource(nil)
	if err != nil {
		t.Fatalf("GenerateResource(nil) returned error: %v", err)
	}
}

func TestGenerateResourceWithAttributes(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test"})

	// Build a schema.Resource via schema.Wrap
	ps := schema.Wrap(&provider.ProviderSchema{
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_widget": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name":  {Type: json.RawMessage(`"string"`), Required: true},
						"count": {Type: json.RawMessage(`"number"`), Optional: true},
					},
				},
			},
		},
	})

	res := ps.Resource("test_widget")
	if res == nil {
		t.Fatal("resource test_widget not found")
	}

	if err := rg.GenerateResource(res); err != nil {
		t.Fatalf("GenerateResource failed: %v", err)
	}

	var buf bytes.Buffer
	if err := rg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TestWidget") {
		t.Error("output does not contain struct name TestWidget")
	}
	if !strings.Contains(output, "Name") {
		t.Error("output does not contain field Name")
	}
}

func TestGenerateResourceWithNestedBlock(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test"})

	ps := schema.Wrap(&provider.ProviderSchema{
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_server": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name": {Type: json.RawMessage(`"string"`), Required: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"disk": {
							NestingMode: "list",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"size": {Type: json.RawMessage(`"number"`), Required: true},
								},
							},
						},
					},
				},
			},
		},
	})

	res := ps.Resource("test_server")
	if res == nil {
		t.Fatal("resource test_server not found")
	}

	if err := rg.GenerateResource(res); err != nil {
		t.Fatalf("GenerateResource failed: %v", err)
	}

	var buf bytes.Buffer
	if err := rg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TestServer") {
		t.Error("output does not contain struct name TestServer")
	}
	if !strings.Contains(output, "DiskBlock") {
		t.Error("output does not contain nested type DiskBlock")
	}
	if !strings.Contains(output, "NewTestServer") {
		t.Error("output does not contain constructor NewTestServer")
	}
}

func TestGenerateResourceWithMultipleNestingModes(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test", OmitEmpty: true})

	ps := schema.Wrap(&provider.ProviderSchema{
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_vm": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name": {Type: json.RawMessage(`"string"`), Required: true},
						"id":   {Type: json.RawMessage(`"string"`), Computed: true},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"network": {
							NestingMode: "single",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"ip": {Type: json.RawMessage(`"string"`), Optional: true},
								},
							},
						},
						"disk": {
							NestingMode: "list",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"size": {Type: json.RawMessage(`"number"`), Required: true},
								},
							},
						},
						"tags": {
							NestingMode: "set",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"key": {Type: json.RawMessage(`"string"`), Required: true},
								},
							},
						},
						"labels": {
							NestingMode: "map",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"value": {Type: json.RawMessage(`"string"`), Optional: true},
								},
							},
						},
						"group": {
							NestingMode: "group",
							Block: &provider.BlockType{
								Attributes: map[string]*provider.Attribute{
									"priority": {Type: json.RawMessage(`"number"`), Optional: true},
								},
							},
						},
					},
				},
			},
		},
	})

	res := ps.Resource("test_vm")
	if res == nil {
		t.Fatal("resource test_vm not found")
	}

	if err := rg.GenerateResource(res); err != nil {
		t.Fatalf("GenerateResource failed: %v", err)
	}

	var buf bytes.Buffer
	if err := rg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"TestVM",
		"NetworkBlock",
		"DiskBlock",
		"TagsBlock",
		"LabelsBlock",
		"GroupBlock",
		"NewTestVM",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
}

func TestGenerateResourceWithDeprecatedAttribute(t *testing.T) {
	rg := NewResourceGenerator(Config{PackageName: "test"})

	ps := schema.Wrap(&provider.ProviderSchema{
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"test_legacy": {
				Block: &provider.BlockType{
					Attributes: map[string]*provider.Attribute{
						"name":     {Type: json.RawMessage(`"string"`), Required: true},
						"old_name": {Type: json.RawMessage(`"string"`), Optional: true, Deprecated: true},
					},
				},
			},
		},
	})

	res := ps.Resource("test_legacy")
	if res == nil {
		t.Fatal("resource test_legacy not found")
	}

	if err := rg.GenerateResource(res); err != nil {
		t.Fatalf("GenerateResource failed: %v", err)
	}

	var buf bytes.Buffer
	if err := rg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TestLegacy") {
		t.Error("output does not contain struct name TestLegacy")
	}
	if !strings.Contains(output, "OldName") {
		t.Error("output does not contain field OldName")
	}
}

func TestJenTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"string", "string"},
		{"float64", "float64"},
		{"int", "int"},
		{"bool", "bool"},
		{"any", "any"},
		{"[]string", "[]string"},
		{"map[string]any", "map[string]any"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			stmt := jenTypeFromString(tt.input)

			// Render via a temporary jen file to get the type as a string
			f := jen.NewFile("tmp")
			f.Var().Id("x").Add(stmt)
			var buf bytes.Buffer
			if err := f.Render(&buf); err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("jenTypeFromString(%q) rendered as %q, want to contain %q", tt.input, output, tt.want)
			}
		})
	}
}
