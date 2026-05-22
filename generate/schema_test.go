package generate

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DataDog/tforge/provider"
)

// newTestProviderSchema builds a fixture ProviderSchema for tests.
func newTestProviderSchema() *provider.ProviderSchema {
	return &provider.ProviderSchema{
		ConfigSchema: &provider.SchemaBlock{
			Block: &provider.BlockType{
				Description: "provider config",
				Attributes: map[string]*provider.Attribute{
					"api_key": {
						Type:        json.RawMessage(`"string"`),
						Description: "API key",
						Required:    true,
						Sensitive:   true,
					},
				},
			},
		},
		ResourceSchemas: map[string]*provider.SchemaBlock{
			"example_thing": {
				Version: 1,
				Block: &provider.BlockType{
					Description: "A thing resource",
					Attributes: map[string]*provider.Attribute{
						"id": {
							Type:     json.RawMessage(`"string"`),
							Computed: true,
						},
						"name": {
							Type:        json.RawMessage(`"string"`),
							Description: "The name",
							Required:    true,
						},
						"count": {
							Type:     json.RawMessage(`"number"`),
							Optional: true,
							Computed: true,
						},
						"enabled": {
							Type:     json.RawMessage(`"bool"`),
							Optional: true,
						},
						"tags": {
							Type:     json.RawMessage(`["list","string"]`),
							Optional: true,
						},
						"metadata": {
							Type:     json.RawMessage(`["map","string"]`),
							Optional: true,
						},
						"old_field": {
							Type:       json.RawMessage(`"string"`),
							Optional:   true,
							Deprecated: true,
						},
						"nested_attr": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"inner_name": {
										Type:     json.RawMessage(`"string"`),
										Required: true,
									},
								},
								NestingMode: provider.NestingModeSingle,
							},
							Optional: true,
						},
						"nested_list_attr": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"value": {
										Type:     json.RawMessage(`"string"`),
										Required: true,
									},
								},
								NestingMode: provider.NestingModeList,
							},
							Optional: true,
						},
						"nested_set_attr": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"key": {
										Type:     json.RawMessage(`"string"`),
										Required: true,
									},
									"value": {
										Type:     json.RawMessage(`"string"`),
										Optional: true,
									},
								},
								NestingMode: provider.NestingModeSet,
							},
							Optional: true,
						},
						"nested_map_attr": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"setting": {
										Type:     json.RawMessage(`"string"`),
										Required: true,
									},
								},
								NestingMode: provider.NestingModeMap,
							},
							Optional: true,
						},
						"nested_unknown_mode": {
							NestedType: &provider.NestedType{
								Attributes: map[string]*provider.Attribute{
									"x": {
										Type:     json.RawMessage(`"string"`),
										Optional: true,
									},
								},
								NestingMode: "unknown_mode",
							},
							Optional: true,
						},
						"nested_empty_attrs": {
							NestedType: &provider.NestedType{
								Attributes:  map[string]*provider.Attribute{},
								NestingMode: provider.NestingModeSingle,
							},
							Optional: true,
						},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"config_block": {
							Block: &provider.BlockType{
								Description: "A config block",
								Attributes: map[string]*provider.Attribute{
									"setting": {
										Type:     json.RawMessage(`"string"`),
										Required: true,
									},
								},
							},
							NestingMode: provider.NestingModeList,
							MinItems:    1,
							MaxItems:    5,
						},
						"single_block": {
							Block: &provider.BlockType{
								Description: "A single block",
							},
							NestingMode: provider.NestingModeSingle,
						},
					},
				},
			},
			"example_other": {
				Block: &provider.BlockType{
					Description: "Another resource",
				},
			},
			"example_deep": {
				Block: &provider.BlockType{
					Description: "A deeply nested resource",
					Attributes: map[string]*provider.Attribute{
						"name": {
							Type:     json.RawMessage(`"string"`),
							Required: true,
						},
					},
					BlockTypes: map[string]*provider.NestedBlock{
						"level_one": {
							Block: &provider.BlockType{
								Description: "First level block",
								Attributes: map[string]*provider.Attribute{
									"l1_attr": {
										Type:     json.RawMessage(`"string"`),
										Optional: true,
									},
								},
								BlockTypes: map[string]*provider.NestedBlock{
									"level_two": {
										Block: &provider.BlockType{
											Description: "Second level block",
											Attributes: map[string]*provider.Attribute{
												"l2_attr": {
													Type:     json.RawMessage(`"number"`),
													Optional: true,
												},
											},
											BlockTypes: map[string]*provider.NestedBlock{
												"level_three": {
													Block: &provider.BlockType{
														Description: "Third level block",
														Attributes: map[string]*provider.Attribute{
															"l3_attr": {
																Type:     json.RawMessage(`"bool"`),
																Required: true,
															},
														},
													},
													NestingMode: provider.NestingModeSingle,
												},
											},
										},
										NestingMode: provider.NestingModeList,
									},
								},
							},
							NestingMode: provider.NestingModeList,
						},
					},
				},
			},
		},
		DataSourceSchemas: map[string]*provider.SchemaBlock{
			"example_things": {
				Block: &provider.BlockType{
					Description: "A data source",
					Attributes: map[string]*provider.Attribute{
						"filter": {
							Type:     json.RawMessage(`"string"`),
							Optional: true,
						},
					},
				},
			},
			"example_info": {
				Block: &provider.BlockType{
					Description: "Info data source",
				},
			},
		},
	}
}

func TestParseTypeJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"string", `"string"`, "string"},
		{"number", `"number"`, "number"},
		{"bool", `"bool"`, "bool"},
		{"list of string", `["list","string"]`, "list(string)"},
		{"set of number", `["set","number"]`, "set(number)"},
		{"map of bool", `["map","bool"]`, "map(bool)"},
		{"object", `["object"]`, "object"},
		{"tuple", `["tuple"]`, "tuple"},
		{"empty", ``, "dynamic"},
		{"nested list", `["list",["list","string"]]`, "list(list(string))"},
		{"array with non-string first element", `[123]`, "dynamic"},
		{"deeply nested map of list", `["map",["list","string"]]`, "map(list(string))"},
		{"list with no inner type", `["list"]`, "list(dynamic)"},
		{"set with no inner type", `["set"]`, "set(dynamic)"},
		{"map with no inner type", `["map"]`, "map(dynamic)"},
		{"number literal", `42`, "dynamic"},
		{"boolean literal", `true`, "dynamic"},
		{"unknown type kind", `["unknown_kind","string"]`, "dynamic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTypeJSON(json.RawMessage(tt.json))
			require.Equal(t, tt.want, got, "parseTypeJSON(%s) = %q, want %q", tt.json, got, tt.want)
		})
	}
}

func TestParseTypeJSONNil(t *testing.T) {
	got := parseTypeJSON(nil)
	require.Equal(t, "dynamic", got, "parseTypeJSON(nil) = %q, want %q", got, "dynamic")
}

func TestGoTypeName(t *testing.T) {
	tests := []struct {
		tfType string
		want   string
	}{
		{"string", "string"},
		{"number", "float64"},
		{"bool", "bool"},
		{"dynamic", "any"},
		{"list(string)", "[]string"},
		{"set(number)", "[]float64"},
		{"map(bool)", "map[string]bool"},
		{"object", "map[string]any"},
		{"tuple", "[]any"},
		{"list(list(string))", "[][]string"},
	}
	for _, tt := range tests {
		t.Run(tt.tfType, func(t *testing.T) {
			got := goTypeName(tt.tfType)
			require.Equal(t, tt.want, got, "goTypeName(%q) = %q, want %q", tt.tfType, got, tt.want)
		})
	}
}

func TestAttributeType(t *testing.T) {
	ps := newTestProviderSchema()
	block := ps.ResourceSchemas["example_thing"].Block

	tests := []struct {
		attrName string
		wantType string
	}{
		{"id", "string"},
		{"name", "string"},
		{"count", "number"},
		{"enabled", "bool"},
		{"tags", "list(string)"},
		{"metadata", "map(string)"},
	}
	for _, tt := range tests {
		attr := block.Attributes[tt.attrName]
		require.NotNil(t, attr, "attribute %q not found", tt.attrName)
		got := attributeType(attr)
		require.Equal(t, tt.wantType, got, "attributeType(%q) = %q, want %q", tt.attrName, got, tt.wantType)
	}
}

func TestAttributeTypeNil(t *testing.T) {
	got := attributeType(nil)
	require.Equal(t, "dynamic", got, "attributeType(nil) = %q, want %q", got, "dynamic")
}

func TestAttributeTypeNested(t *testing.T) {
	ps := newTestProviderSchema()
	block := ps.ResourceSchemas["example_thing"].Block

	tests := []struct {
		attrName string
		wantType string
	}{
		{"nested_attr", "object"},
		{"nested_list_attr", "list(object)"},
		{"nested_set_attr", "set(object)"},
		{"nested_map_attr", "map(object)"},
		{"nested_unknown_mode", "object"},
	}
	for _, tt := range tests {
		attr := block.Attributes[tt.attrName]
		require.NotNil(t, attr, "attribute %q not found", tt.attrName)
		got := attributeType(attr)
		require.Equal(t, tt.wantType, got, "attributeType(%q) = %q, want %q", tt.attrName, got, tt.wantType)
	}
}

func TestSortedSchemas_Resources(t *testing.T) {
	ps := newTestProviderSchema()
	resources := sortedSchemas(ps.ResourceSchemas)
	require.Len(t, resources, 3, "expected 3 resources, got %d", len(resources))
	require.Equal(t, "example_deep", resources[0].Name, "expected first resource %q, got %q", "example_deep", resources[0].Name)
	require.Equal(t, "example_other", resources[1].Name, "expected second resource %q, got %q", "example_other", resources[1].Name)
	require.Equal(t, "example_thing", resources[2].Name, "expected third resource %q, got %q", "example_thing", resources[2].Name)
}

func TestSortedSchemas_Nil(t *testing.T) {
	require.Nil(t, sortedSchemas(nil), "expected nil for nil map")
}

func TestSortedSchemas_DataSources(t *testing.T) {
	ps := newTestProviderSchema()
	dataSources := sortedSchemas(ps.DataSourceSchemas)
	require.Len(t, dataSources, 2, "expected 2 data sources, got %d", len(dataSources))
	require.Equal(t, "example_info", dataSources[0].Name, "expected first data source %q, got %q", "example_info", dataSources[0].Name)
	require.Equal(t, "example_things", dataSources[1].Name, "expected second data source %q, got %q", "example_things", dataSources[1].Name)
}

func TestSortedAttributes(t *testing.T) {
	ps := newTestProviderSchema()
	block := ps.ResourceSchemas["example_thing"].Block

	attrs := sortedAttributes(block)
	require.NotEmpty(t, attrs, "expected attributes, got none")
	// Check sorted order
	for i := 1; i < len(attrs); i++ {
		require.Less(t, attrs[i-1].Name, attrs[i].Name, "attributes not sorted: %q >= %q", attrs[i-1].Name, attrs[i].Name)
	}
}

func TestSortedAttributesNil(t *testing.T) {
	require.Nil(t, sortedAttributes(nil), "expected nil for nil block type")
}

func TestSortedNestedBlocks(t *testing.T) {
	ps := newTestProviderSchema()
	block := ps.ResourceSchemas["example_thing"].Block

	nbs := sortedNestedBlocks(block)
	require.Len(t, nbs, 2, "expected 2 nested blocks, got %d", len(nbs))
	require.Equal(t, "config_block", nbs[0].Name, "expected first nested block %q, got %q", "config_block", nbs[0].Name)
	require.Equal(t, "single_block", nbs[1].Name, "expected second nested block %q, got %q", "single_block", nbs[1].Name)
}

func TestSortedNestedBlocksNil(t *testing.T) {
	require.Nil(t, sortedNestedBlocks(nil), "expected nil for nil block type")
}

func TestExtractInnerType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"list of string", "list(string)", "string"},
		{"map of list of string", "map(list(string))", "list(string)"},
		{"no parens", "string", "any"},
		{"empty string", "", "any"},
		{"empty parens", "list()", ""},
		{"missing close paren", "list(string", "any"},
		{"missing open paren", "list)string(", "any"},
		{"close before open", ")string(", "any"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInnerType(tt.input)
			require.Equal(t, tt.want, got, "extractInnerType(%q) = %q, want %q", tt.input, got, tt.want)
		})
	}
}

func TestNestedTypeString(t *testing.T) {
	tests := []struct {
		mode provider.NestingMode
		want string
	}{
		{provider.NestingModeSingle, "object"},
		{provider.NestingModeList, "list(object)"},
		{provider.NestingModeSet, "set(object)"},
		{provider.NestingModeMap, "map(object)"},
		{"unknown", "object"},
	}
	for _, tt := range tests {
		nt := &provider.NestedType{NestingMode: tt.mode}
		got := nestedTypeString(nt)
		require.Equal(t, tt.want, got, "nestedTypeString(%q) = %q, want %q", tt.mode, got, tt.want)
	}

	// nil nested type
	got := nestedTypeString(nil)
	require.Equal(t, "dynamic", got, "nestedTypeString(nil) = %q, want %q", got, "dynamic")
}
