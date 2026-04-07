package schema

import (
	"encoding/json"
	"testing"

	"tforge/provider"
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
								NestingMode: "single",
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
								NestingMode: "list",
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
							NestingMode: "list",
							MinItems:    1,
							MaxItems:    5,
						},
						"single_block": {
							Block: &provider.BlockType{
								Description: "A single block",
							},
							NestingMode: "single",
						},
					},
				},
			},
			"example_other": {
				Block: &provider.BlockType{
					Description: "Another resource",
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

func TestWrapAndRaw(t *testing.T) {
	ps := newTestProviderSchema()
	wrapped := Wrap(ps)
	if wrapped == nil {
		t.Fatal("Wrap returned nil")
	}
	if wrapped.Raw() != ps {
		t.Error("Raw() did not return the same pointer")
	}
}

func TestProviderBlock(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.ProviderBlock()
	if block == nil {
		t.Fatal("ProviderBlock returned nil")
	}
	if block.Name() != "provider" {
		t.Errorf("expected name %q, got %q", "provider", block.Name())
	}
	if block.Description() != "provider config" {
		t.Errorf("expected description %q, got %q", "provider config", block.Description())
	}
}

func TestProviderBlockNil(t *testing.T) {
	ps := Wrap(&provider.ProviderSchema{})
	if ps.ProviderBlock() != nil {
		t.Error("expected nil for missing ConfigSchema")
	}
}

func TestResource(t *testing.T) {
	ps := Wrap(newTestProviderSchema())

	r := ps.Resource("example_thing")
	if r == nil {
		t.Fatal("Resource returned nil for existing resource")
	}
	if r.Name() != "example_thing" {
		t.Errorf("expected name %q, got %q", "example_thing", r.Name())
	}
	if r.Description() != "A thing resource" {
		t.Errorf("expected description %q, got %q", "A thing resource", r.Description())
	}
	if r.Block() == nil {
		t.Error("Block() returned nil")
	}
}

func TestResourceNotFound(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	if ps.Resource("nonexistent") != nil {
		t.Error("expected nil for nonexistent resource")
	}
}

func TestDataSource(t *testing.T) {
	ps := Wrap(newTestProviderSchema())

	ds := ps.DataSource("example_things")
	if ds == nil {
		t.Fatal("DataSource returned nil for existing data source")
	}
	if ds.Name() != "example_things" {
		t.Errorf("expected name %q, got %q", "example_things", ds.Name())
	}
	if ds.Description() != "A data source" {
		t.Errorf("expected description %q, got %q", "A data source", ds.Description())
	}
	if ds.Block() == nil {
		t.Error("Block() returned nil")
	}
}

func TestDataSourceNotFound(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	if ps.DataSource("nonexistent") != nil {
		t.Error("expected nil for nonexistent data source")
	}
}

func TestResources(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	resources := ps.Resources()
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if resources[0].Name() != "example_other" {
		t.Errorf("expected first resource %q, got %q", "example_other", resources[0].Name())
	}
	if resources[1].Name() != "example_thing" {
		t.Errorf("expected second resource %q, got %q", "example_thing", resources[1].Name())
	}
}

func TestResourcesNil(t *testing.T) {
	ps := Wrap(&provider.ProviderSchema{})
	if ps.Resources() != nil {
		t.Error("expected nil for empty ResourceSchemas")
	}
}

func TestDataSources(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	dataSources := ps.DataSources()
	if len(dataSources) != 2 {
		t.Fatalf("expected 2 data sources, got %d", len(dataSources))
	}
	if dataSources[0].Name() != "example_info" {
		t.Errorf("expected first data source %q, got %q", "example_info", dataSources[0].Name())
	}
	if dataSources[1].Name() != "example_things" {
		t.Errorf("expected second data source %q, got %q", "example_things", dataSources[1].Name())
	}
}

func TestDataSourcesNil(t *testing.T) {
	ps := Wrap(&provider.ProviderSchema{})
	if ps.DataSources() != nil {
		t.Error("expected nil for empty DataSourceSchemas")
	}
}

func TestBlockAttributes(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	attrs := block.Attributes()
	if len(attrs) == 0 {
		t.Fatal("expected attributes, got none")
	}
	// Check sorted order
	for i := 1; i < len(attrs); i++ {
		if attrs[i-1].Name() >= attrs[i].Name() {
			t.Errorf("attributes not sorted: %q >= %q", attrs[i-1].Name(), attrs[i].Name())
		}
	}
}

func TestBlockAttribute(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	attr := block.Attribute("name")
	if attr == nil {
		t.Fatal("Attribute returned nil for existing attribute")
	}
	if attr.Name() != "name" {
		t.Errorf("expected name %q, got %q", "name", attr.Name())
	}
	if attr.Description() != "The name" {
		t.Errorf("expected description %q, got %q", "The name", attr.Description())
	}

	if block.Attribute("nonexistent") != nil {
		t.Error("expected nil for nonexistent attribute")
	}
}

func TestAttributeFlags(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	// Required attribute
	name := block.Attribute("name")
	if !name.Required() {
		t.Error("expected name to be required")
	}
	if name.Optional() {
		t.Error("expected name not to be optional")
	}
	if name.Computed() {
		t.Error("expected name not to be computed")
	}

	// Optional + Computed attribute
	count := block.Attribute("count")
	if count.Required() {
		t.Error("expected count not to be required")
	}
	if !count.Optional() {
		t.Error("expected count to be optional")
	}
	if !count.Computed() {
		t.Error("expected count to be computed")
	}

	// Sensitive attribute
	apiKey := ps.ProviderBlock().Attribute("api_key")
	if !apiKey.Sensitive() {
		t.Error("expected api_key to be sensitive")
	}

	// Deprecated attribute
	oldField := block.Attribute("old_field")
	if oldField.Deprecated() == "" {
		t.Error("expected old_field to be deprecated")
	}

	// Non-deprecated
	if name.Deprecated() != "" {
		t.Error("expected name not to be deprecated")
	}
}

func TestAttributeType(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

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
		attr := block.Attribute(tt.attrName)
		if attr == nil {
			t.Errorf("attribute %q not found", tt.attrName)
			continue
		}
		if got := attr.Type(); got != tt.wantType {
			t.Errorf("Attribute(%q).Type() = %q, want %q", tt.attrName, got, tt.wantType)
		}
	}
}

func TestAttributeNestedType(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	// single nested type
	nested := block.Attribute("nested_attr")
	if nested == nil {
		t.Fatal("nested_attr not found")
	}
	if !nested.HasNestedType() {
		t.Error("expected HasNestedType to be true")
	}
	if got := nested.Type(); got != "object" {
		t.Errorf("expected type %q, got %q", "object", got)
	}
	innerAttrs := nested.NestedAttributes()
	if len(innerAttrs) != 1 {
		t.Fatalf("expected 1 nested attribute, got %d", len(innerAttrs))
	}
	if innerAttrs[0].Name() != "inner_name" {
		t.Errorf("expected nested attribute %q, got %q", "inner_name", innerAttrs[0].Name())
	}

	// list nested type
	nestedList := block.Attribute("nested_list_attr")
	if nestedList == nil {
		t.Fatal("nested_list_attr not found")
	}
	if got := nestedList.Type(); got != "list(object)" {
		t.Errorf("expected type %q, got %q", "list(object)", got)
	}

	// non-nested attribute
	name := block.Attribute("name")
	if name.HasNestedType() {
		t.Error("expected HasNestedType to be false for name")
	}
	if name.NestedAttributes() != nil {
		t.Error("expected nil NestedAttributes for name")
	}
}

func TestNestedBlocks(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	nbs := block.NestedBlocks()
	if len(nbs) != 2 {
		t.Fatalf("expected 2 nested blocks, got %d", len(nbs))
	}
	// Sorted by name
	if nbs[0].Name() != "config_block" {
		t.Errorf("expected first nested block %q, got %q", "config_block", nbs[0].Name())
	}
	if nbs[1].Name() != "single_block" {
		t.Errorf("expected second nested block %q, got %q", "single_block", nbs[1].Name())
	}
}

func TestNestedBlock(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	nb := block.NestedBlock("config_block")
	if nb == nil {
		t.Fatal("NestedBlock returned nil for existing block")
	}
	if nb.Name() != "config_block" {
		t.Errorf("expected name %q, got %q", "config_block", nb.Name())
	}
	if nb.NestingMode() != "list" {
		t.Errorf("expected nesting mode %q, got %q", "list", nb.NestingMode())
	}
	if nb.Description() != "A config block" {
		t.Errorf("expected description %q, got %q", "A config block", nb.Description())
	}
	if nb.MinItems() != 1 {
		t.Errorf("expected MinItems 1, got %d", nb.MinItems())
	}
	if nb.MaxItems() != 5 {
		t.Errorf("expected MaxItems 5, got %d", nb.MaxItems())
	}
	if nb.Block() == nil {
		t.Error("Block() returned nil")
	}

	// Nonexistent
	if block.NestedBlock("nonexistent") != nil {
		t.Error("expected nil for nonexistent nested block")
	}
}

func TestNestedBlockGoType(t *testing.T) {
	ps := Wrap(newTestProviderSchema())
	block := ps.Resource("example_thing").Block()

	tests := []struct {
		blockName string
		typeName  string
		want      string
	}{
		{"config_block", "ConfigBlock", "[]*ConfigBlock"},
		{"single_block", "SingleBlock", "*SingleBlock"},
	}
	for _, tt := range tests {
		nb := block.NestedBlock(tt.blockName)
		if nb == nil {
			t.Errorf("nested block %q not found", tt.blockName)
			continue
		}
		if got := nb.GoType(tt.typeName); got != tt.want {
			t.Errorf("GoType(%q) for %q = %q, want %q", tt.typeName, tt.blockName, got, tt.want)
		}
	}

	// Test map and group modes with a custom nested block
	mapNB := &NestedBlock{name: "m", raw: &provider.NestedBlock{NestingMode: "map"}}
	if got := mapNB.GoType("Foo"); got != "map[string]*Foo" {
		t.Errorf("GoType for map mode = %q, want %q", got, "map[string]*Foo")
	}
	groupNB := &NestedBlock{name: "g", raw: &provider.NestedBlock{NestingMode: "group"}}
	if got := groupNB.GoType("Bar"); got != "*Bar" {
		t.Errorf("GoType for group mode = %q, want %q", got, "*Bar")
	}
	unknownNB := &NestedBlock{name: "u", raw: &provider.NestedBlock{NestingMode: "unknown"}}
	if got := unknownNB.GoType("Baz"); got != "Baz" {
		t.Errorf("GoType for unknown mode = %q, want %q", got, "Baz")
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTypeJSON(json.RawMessage(tt.json))
			if got != tt.want {
				t.Errorf("parseTypeJSON(%s) = %q, want %q", tt.json, got, tt.want)
			}
		})
	}
}

func TestParseTypeJSONNil(t *testing.T) {
	got := parseTypeJSON(nil)
	if got != "dynamic" {
		t.Errorf("parseTypeJSON(nil) = %q, want %q", got, "dynamic")
	}
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
			got := GoTypeName(tt.tfType)
			if got != tt.want {
				t.Errorf("GoTypeName(%q) = %q, want %q", tt.tfType, got, tt.want)
			}
		})
	}
}

func TestAttributeTypeNilRaw(t *testing.T) {
	a := &Attribute{name: "x", raw: nil}
	if got := a.Type(); got != "dynamic" {
		t.Errorf("expected %q, got %q", "dynamic", got)
	}
	if a.Required() {
		t.Error("nil raw should not be required")
	}
	if a.Optional() {
		t.Error("nil raw should not be optional")
	}
	if a.Computed() {
		t.Error("nil raw should not be computed")
	}
	if a.Sensitive() {
		t.Error("nil raw should not be sensitive")
	}
	if a.Deprecated() != "" {
		t.Error("nil raw should not be deprecated")
	}
	if a.Description() != "" {
		t.Error("nil raw should have empty description")
	}
	if a.HasNestedType() {
		t.Error("nil raw should not have nested type")
	}
}

func TestBlockNilRaw(t *testing.T) {
	b := &Block{name: "x", raw: nil}
	if b.Description() != "" {
		t.Error("nil raw block should have empty description")
	}
	if b.Attributes() != nil {
		t.Error("nil raw block should return nil attributes")
	}
	if b.Attribute("foo") != nil {
		t.Error("nil raw block should return nil for Attribute")
	}
	if b.NestedBlocks() != nil {
		t.Error("nil raw block should return nil nested blocks")
	}
	if b.NestedBlock("foo") != nil {
		t.Error("nil raw block should return nil for NestedBlock")
	}
}

func TestNestedBlockNilRaw(t *testing.T) {
	nb := &NestedBlock{name: "x", raw: nil}
	if nb.NestingMode() != "" {
		t.Error("nil raw should have empty nesting mode")
	}
	if nb.Description() != "" {
		t.Error("nil raw should have empty description")
	}
	if nb.Block() != nil {
		t.Error("nil raw should return nil block")
	}
	if nb.MinItems() != 0 {
		t.Error("nil raw should return 0 min items")
	}
	if nb.MaxItems() != 0 {
		t.Error("nil raw should return 0 max items")
	}
}

func TestResourceNilBlock(t *testing.T) {
	r := &Resource{name: "x", raw: nil}
	if r.Description() != "" {
		t.Error("nil raw should have empty description")
	}
	if r.Block() != nil {
		t.Error("nil raw should return nil block")
	}
}

func TestDataSourceNilBlock(t *testing.T) {
	ds := &DataSource{name: "x", raw: nil}
	if ds.Description() != "" {
		t.Error("nil raw should have empty description")
	}
	if ds.Block() != nil {
		t.Error("nil raw should return nil block")
	}
}
