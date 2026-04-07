package generate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"tforge/provider"
	"tforge/schema"
)

func TestNewProviderGenerator(t *testing.T) {
	pg := NewProviderGenerator(Config{PackageName: "test"})
	if pg == nil {
		t.Fatal("NewProviderGenerator returned nil")
	}
	if pg.File() == nil {
		t.Fatal("File() returned nil")
	}
}

func TestGenerateProvider(t *testing.T) {
	pg := NewProviderGenerator(Config{PackageName: "test"})

	ps := schema.Wrap(&provider.ProviderSchema{
		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{
				Attributes: map[string]*provider.Attribute{
					"api_key":  {Type: json.RawMessage(`"string"`), Required: true},
					"base_url": {Type: json.RawMessage(`"string"`), Optional: true},
				},
			},
		},
	})

	if err := pg.GenerateProvider(ps, "test"); err != nil {
		t.Fatalf("GenerateProvider failed: %v", err)
	}

	var buf bytes.Buffer
	if err := pg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"ProviderConfig", "ProviderName", "NewProvider"} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
}

func TestGenerateProviderNilBlock(t *testing.T) {
	pg := NewProviderGenerator(Config{PackageName: "test"})

	ps := schema.Wrap(&provider.ProviderSchema{
		ConfigSchema: nil,
	})

	if err := pg.GenerateProvider(ps, "test"); err != nil {
		t.Fatalf("GenerateProvider with nil block failed: %v", err)
	}

	var buf bytes.Buffer
	if err := pg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()

	// Should still generate empty struct and helpers
	if !strings.Contains(output, "ProviderConfig") {
		t.Error("output does not contain ProviderConfig")
	}
	if !strings.Contains(output, "NewProvider") {
		t.Error("output does not contain NewProvider")
	}
	if !strings.Contains(output, "ProviderName") {
		t.Error("output does not contain ProviderName")
	}
}

func TestGenerateProviderWithNestedBlocks(t *testing.T) {
	pg := NewProviderGenerator(Config{PackageName: "test", OmitEmpty: true})

	ps := schema.Wrap(&provider.ProviderSchema{
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
								"retries": {Type: json.RawMessage(`"number"`), Optional: true},
							},
						},
					},
					"endpoints": {
						NestingMode: "list",
						Block: &provider.BlockType{
							Attributes: map[string]*provider.Attribute{
								"url": {Type: json.RawMessage(`"string"`), Required: true},
							},
						},
					},
				},
			},
		},
	})

	if err := pg.GenerateProvider(ps, "testprovider"); err != nil {
		t.Fatalf("GenerateProvider failed: %v", err)
	}

	var buf bytes.Buffer
	if err := pg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()

	// Check provider struct and nested block types
	for _, want := range []string{
		"ProviderConfig",
		"SettingsBlock",
		"EndpointsBlock",
		"APIKey",
		"Timeout",
		"Retries",
		"URL",
		"NewProvider",
		"ProviderName",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
}

func TestGenerateProviderWithMapNesting(t *testing.T) {
	pg := NewProviderGenerator(Config{PackageName: "test"})

	ps := schema.Wrap(&provider.ProviderSchema{
		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{
				Attributes: map[string]*provider.Attribute{
					"name": {Type: json.RawMessage(`"string"`), Required: true},
				},
				BlockTypes: map[string]*provider.NestedBlock{
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
	})

	if err := pg.GenerateProvider(ps, "mapprov"); err != nil {
		t.Fatalf("GenerateProvider failed: %v", err)
	}

	var buf bytes.Buffer
	if err := pg.File().Render(&buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"LabelsBlock", "GroupBlock", "ProviderConfig"} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
}
