// Package schema provides utilities for navigating Terraform provider schemas.
//
// This package wraps the provider schema types with helper methods for:
//   - Navigating resource blocks and attributes
//   - Handling nested block structures
//   - Converting between schema types and Go types
//
// Example usage:
//
//	ps := schema.Wrap(providerSchema)
//	for _, res := range ps.Resources() {
//	    fmt.Println(res.Name, res.Description)
//	}
package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"tforge/provider"
)

// ProviderSchema wraps a provider.ProviderSchema with navigation helpers.
type ProviderSchema struct {
	raw *provider.ProviderSchema
}

// Wrap creates a new ProviderSchema wrapper.
func Wrap(ps *provider.ProviderSchema) *ProviderSchema {
	return &ProviderSchema{raw: ps}
}

// Raw returns the underlying provider schema.
func (ps *ProviderSchema) Raw() *provider.ProviderSchema {
	return ps.raw
}

// ProviderBlock returns the provider configuration block schema.
func (ps *ProviderSchema) ProviderBlock() *Block {
	if ps.raw == nil || ps.raw.ConfigSchema == nil {
		return nil
	}
	return &Block{raw: ps.raw.ConfigSchema.Block, name: "provider"}
}

// Resource returns the schema for a specific resource type.
func (ps *ProviderSchema) Resource(name string) *Resource {
	if ps.raw == nil || ps.raw.ResourceSchemas == nil {
		return nil
	}
	rs, ok := ps.raw.ResourceSchemas[name]
	if !ok {
		return nil
	}
	return &Resource{
		name: name,
		raw:  rs,
	}
}

// DataSource returns the schema for a specific data source type.
func (ps *ProviderSchema) DataSource(name string) *DataSource {
	if ps.raw == nil || ps.raw.DataSourceSchemas == nil {
		return nil
	}
	ds, ok := ps.raw.DataSourceSchemas[name]
	if !ok {
		return nil
	}
	return &DataSource{
		name: name,
		raw:  ds,
	}
}

// Resources returns all resource schemas sorted by name.
func (ps *ProviderSchema) Resources() []*Resource {
	if ps.raw == nil || ps.raw.ResourceSchemas == nil {
		return nil
	}

	resources := make([]*Resource, 0, len(ps.raw.ResourceSchemas))
	for name, rs := range ps.raw.ResourceSchemas {
		resources = append(resources, &Resource{name: name, raw: rs})
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].name < resources[j].name
	})
	return resources
}

// DataSources returns all data source schemas sorted by name.
func (ps *ProviderSchema) DataSources() []*DataSource {
	if ps.raw == nil || ps.raw.DataSourceSchemas == nil {
		return nil
	}

	dataSources := make([]*DataSource, 0, len(ps.raw.DataSourceSchemas))
	for name, ds := range ps.raw.DataSourceSchemas {
		dataSources = append(dataSources, &DataSource{name: name, raw: ds})
	}

	sort.Slice(dataSources, func(i, j int) bool {
		return dataSources[i].name < dataSources[j].name
	})
	return dataSources
}

// Resource represents a Terraform resource schema.
type Resource struct {
	name string
	raw  *provider.SchemaBlock
}

// Name returns the resource type name (e.g., "datadog_monitor").
func (r *Resource) Name() string { return r.name }

// Description returns the resource description.
func (r *Resource) Description() string {
	if r.raw == nil || r.raw.Block == nil {
		return ""
	}
	return r.raw.Block.Description
}

// Block returns the resource's block schema.
func (r *Resource) Block() *Block {
	if r.raw == nil || r.raw.Block == nil {
		return nil
	}
	return &Block{raw: r.raw.Block, name: r.name}
}

// DataSource represents a Terraform data source schema.
type DataSource struct {
	name string
	raw  *provider.SchemaBlock
}

// Name returns the data source type name (e.g., "datadog_monitors").
func (ds *DataSource) Name() string { return ds.name }

// Description returns the data source description.
func (ds *DataSource) Description() string {
	if ds.raw == nil || ds.raw.Block == nil {
		return ""
	}
	return ds.raw.Block.Description
}

// Block returns the data source's block schema.
func (ds *DataSource) Block() *Block {
	if ds.raw == nil || ds.raw.Block == nil {
		return nil
	}
	return &Block{raw: ds.raw.Block, name: ds.name}
}

// Block represents a Terraform block schema (provider config, resource, or nested block).
type Block struct {
	raw  *provider.BlockType
	name string
}

// Name returns the block name.
func (b *Block) Name() string { return b.name }

// Description returns the block description.
func (b *Block) Description() string {
	if b.raw == nil {
		return ""
	}
	return b.raw.Description
}

// Attributes returns all attributes in this block sorted by name.
func (b *Block) Attributes() []*Attribute {
	if b.raw == nil || b.raw.Attributes == nil {
		return nil
	}

	attrs := make([]*Attribute, 0, len(b.raw.Attributes))
	for name, attr := range b.raw.Attributes {
		attrs = append(attrs, &Attribute{name: name, raw: attr})
	}

	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].name < attrs[j].name
	})
	return attrs
}

// Attribute returns a specific attribute by name.
func (b *Block) Attribute(name string) *Attribute {
	if b.raw == nil || b.raw.Attributes == nil {
		return nil
	}
	attr, ok := b.raw.Attributes[name]
	if !ok {
		return nil
	}
	return &Attribute{name: name, raw: attr}
}

// NestedBlocks returns all nested blocks sorted by name.
func (b *Block) NestedBlocks() []*NestedBlock {
	if b.raw == nil || b.raw.BlockTypes == nil {
		return nil
	}

	blocks := make([]*NestedBlock, 0, len(b.raw.BlockTypes))
	for name, nb := range b.raw.BlockTypes {
		blocks = append(blocks, &NestedBlock{name: name, raw: nb})
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].name < blocks[j].name
	})
	return blocks
}

// NestedBlock returns a specific nested block by name.
func (b *Block) NestedBlock(name string) *NestedBlock {
	if b.raw == nil || b.raw.BlockTypes == nil {
		return nil
	}
	nb, ok := b.raw.BlockTypes[name]
	if !ok {
		return nil
	}
	return &NestedBlock{name: name, raw: nb}
}

// Attribute represents a Terraform attribute schema.
type Attribute struct {
	name string
	raw  *provider.Attribute
}

// Name returns the attribute name.
func (a *Attribute) Name() string { return a.name }

// Description returns the attribute description.
func (a *Attribute) Description() string {
	if a.raw == nil {
		return ""
	}
	return a.raw.Description
}

// Type returns the attribute type as a string.
func (a *Attribute) Type() string {
	if a.raw == nil {
		return "dynamic"
	}
	// Parse the type from JSON
	if len(a.raw.Type) == 0 {
		// Check for nested type
		if a.raw.NestedType != nil {
			return a.nestedTypeString()
		}
		return "dynamic"
	}
	return parseTypeJSON(a.raw.Type)
}

// nestedTypeString returns a string representation of a nested type.
func (a *Attribute) nestedTypeString() string {
	if a.raw.NestedType == nil {
		return "dynamic"
	}
	switch a.raw.NestedType.NestingMode {
	case "single":
		return "object"
	case "list":
		return "list(object)"
	case "set":
		return "set(object)"
	case "map":
		return "map(object)"
	default:
		return "object"
	}
}

// Required returns true if this attribute is required.
func (a *Attribute) Required() bool {
	if a.raw == nil {
		return false
	}
	return a.raw.Required
}

// Optional returns true if this attribute is optional.
func (a *Attribute) Optional() bool {
	if a.raw == nil {
		return false
	}
	return a.raw.Optional
}

// Computed returns true if this attribute is computed.
func (a *Attribute) Computed() bool {
	if a.raw == nil {
		return false
	}
	return a.raw.Computed
}

// Sensitive returns true if this attribute is sensitive.
func (a *Attribute) Sensitive() bool {
	if a.raw == nil {
		return false
	}
	return a.raw.Sensitive
}

// Deprecated returns the deprecation message, or empty if not deprecated.
func (a *Attribute) Deprecated() string {
	if a.raw == nil || !a.raw.Deprecated {
		return ""
	}
	return "deprecated"
}

// HasNestedType returns true if this attribute has a nested object type.
func (a *Attribute) HasNestedType() bool {
	return a.raw != nil && a.raw.NestedType != nil
}

// NestedAttributes returns the attributes of a nested type.
func (a *Attribute) NestedAttributes() []*Attribute {
	if a.raw == nil || a.raw.NestedType == nil {
		return nil
	}

	attrs := make([]*Attribute, 0, len(a.raw.NestedType.Attributes))
	for name, attr := range a.raw.NestedType.Attributes {
		attrs = append(attrs, &Attribute{name: name, raw: attr})
	}

	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].name < attrs[j].name
	})
	return attrs
}

// NestedBlock represents a nested block within a Terraform block.
type NestedBlock struct {
	name string
	raw  *provider.NestedBlock
}

// Name returns the nested block name.
func (nb *NestedBlock) Name() string { return nb.name }

// Description returns the nested block description.
func (nb *NestedBlock) Description() string {
	if nb.raw == nil || nb.raw.Block == nil {
		return ""
	}
	return nb.raw.Block.Description
}

// NestingMode returns the nesting mode (single, list, set, map).
func (nb *NestedBlock) NestingMode() string {
	if nb.raw == nil {
		return ""
	}
	return nb.raw.NestingMode
}

// Block returns the nested block's schema.
func (nb *NestedBlock) Block() *Block {
	if nb.raw == nil || nb.raw.Block == nil {
		return nil
	}
	return &Block{raw: nb.raw.Block, name: nb.name}
}

// MinItems returns the minimum number of items (for list/set).
func (nb *NestedBlock) MinItems() int {
	if nb.raw == nil {
		return 0
	}
	return nb.raw.MinItems
}

// MaxItems returns the maximum number of items (for list/set), 0 means unlimited.
func (nb *NestedBlock) MaxItems() int {
	if nb.raw == nil {
		return 0
	}
	return nb.raw.MaxItems
}

// GoType returns the Go type string for this nested block.
func (nb *NestedBlock) GoType(typeName string) string {
	switch nb.NestingMode() {
	case "single", "group":
		return fmt.Sprintf("*%s", typeName)
	case "list", "set":
		return fmt.Sprintf("[]*%s", typeName)
	case "map":
		return fmt.Sprintf("map[string]*%s", typeName)
	default:
		return typeName
	}
}

// parseTypeJSON parses a cty type from JSON.
func parseTypeJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return "dynamic"
	}

	// Try string first (primitive types like "string", "number", "bool")
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		return str
	}

	// Try array for complex types like ["list", "string"]
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) >= 1 {
		var typeKind string
		if err := json.Unmarshal(arr[0], &typeKind); err == nil {
			switch typeKind {
			case "list":
				if len(arr) >= 2 {
					inner := parseTypeJSON(arr[1])
					return fmt.Sprintf("list(%s)", inner)
				}
				return "list(dynamic)"
			case "set":
				if len(arr) >= 2 {
					inner := parseTypeJSON(arr[1])
					return fmt.Sprintf("set(%s)", inner)
				}
				return "set(dynamic)"
			case "map":
				if len(arr) >= 2 {
					inner := parseTypeJSON(arr[1])
					return fmt.Sprintf("map(%s)", inner)
				}
				return "map(dynamic)"
			case "object":
				return "object"
			case "tuple":
				return "tuple"
			}
		}
	}

	return "dynamic"
}

// GoTypeName converts a Terraform attribute type to a Go type name.
func GoTypeName(tfType string) string {
	// Normalize the type string
	tfType = strings.TrimSpace(tfType)

	switch tfType {
	case "string":
		return "string"
	case "number":
		return "float64"
	case "bool":
		return "bool"
	case "dynamic":
		return "any"
	default:
		// Handle complex types
		if strings.HasPrefix(tfType, "list") || strings.HasPrefix(tfType, "set") {
			// Extract inner type
			inner := extractInnerType(tfType)
			return "[]" + GoTypeName(inner)
		}
		if strings.HasPrefix(tfType, "map") {
			inner := extractInnerType(tfType)
			return "map[string]" + GoTypeName(inner)
		}
		if strings.HasPrefix(tfType, "object") {
			return "map[string]any"
		}
		if strings.HasPrefix(tfType, "tuple") {
			return "[]any"
		}
		return "any"
	}
}

// extractInnerType extracts the inner type from a complex type like list(string).
func extractInnerType(tfType string) string {
	start := strings.Index(tfType, "(")
	end := strings.LastIndex(tfType, ")")
	if start == -1 || end == -1 || end <= start {
		return "any"
	}
	return strings.TrimSpace(tfType[start+1 : end])
}
