package generate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"tforge/provider"
)

// resourceEntry pairs a resource name with its schema for sorted iteration.
type resourceEntry struct {
	Name   string
	Schema *provider.SchemaBlock
}

// dataSourceEntry pairs a data source name with its schema for sorted iteration.
type dataSourceEntry struct {
	Name   string
	Schema *provider.SchemaBlock
}

// attributeEntry pairs an attribute name with its schema for sorted iteration.
type attributeEntry struct {
	Name string
	Attr *provider.Attribute
}

// nestedBlockEntry pairs a nested block name with its schema for sorted iteration.
type nestedBlockEntry struct {
	Name  string
	Block *provider.NestedBlock
}

// sortedResources returns all resource schemas sorted by name.
func sortedResources(schemas map[string]*provider.SchemaBlock) []resourceEntry {
	if len(schemas) == 0 {
		return nil
	}
	entries := make([]resourceEntry, 0, len(schemas))
	for name, s := range schemas {
		entries = append(entries, resourceEntry{Name: name, Schema: s})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// sortedDataSources returns all data source schemas sorted by name.
func sortedDataSources(schemas map[string]*provider.SchemaBlock) []dataSourceEntry {
	if len(schemas) == 0 {
		return nil
	}
	entries := make([]dataSourceEntry, 0, len(schemas))
	for name, s := range schemas {
		entries = append(entries, dataSourceEntry{Name: name, Schema: s})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// sortedAttributes returns all attributes in a block type sorted by name.
func sortedAttributes(bt *provider.BlockType) []attributeEntry {
	if bt == nil || len(bt.Attributes) == 0 {
		return nil
	}
	entries := make([]attributeEntry, 0, len(bt.Attributes))
	for name, attr := range bt.Attributes {
		entries = append(entries, attributeEntry{Name: name, Attr: attr})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// sortedNestedBlocks returns all nested blocks in a block type sorted by name.
func sortedNestedBlocks(bt *provider.BlockType) []nestedBlockEntry {
	if bt == nil || len(bt.BlockTypes) == 0 {
		return nil
	}
	entries := make([]nestedBlockEntry, 0, len(bt.BlockTypes))
	for name, nb := range bt.BlockTypes {
		entries = append(entries, nestedBlockEntry{Name: name, Block: nb})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// attributeType returns the type string for an attribute, handling nested types.
func attributeType(attr *provider.Attribute) string {
	if attr == nil {
		return "dynamic"
	}
	if len(attr.Type) == 0 {
		if attr.NestedType != nil {
			return nestedTypeString(attr.NestedType)
		}
		return "dynamic"
	}
	return parseTypeJSON(attr.Type)
}

// nestedTypeString returns a string representation of a nested type.
func nestedTypeString(nt *provider.NestedType) string {
	if nt == nil {
		return "dynamic"
	}
	switch nt.NestingMode {
	case provider.NestingModeSingle:
		return "object"
	case provider.NestingModeList:
		return "list(object)"
	case provider.NestingModeSet:
		return "set(object)"
	case provider.NestingModeMap:
		return "map(object)"
	default:
		return "object"
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

// goTypeName converts a Terraform attribute type to a Go type name.
func goTypeName(tfType string) string {
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
			return "[]" + goTypeName(inner)
		}
		if strings.HasPrefix(tfType, "map") {
			inner := extractInnerType(tfType)
			return "map[string]" + goTypeName(inner)
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
