package generate

import (
	"github.com/dave/jennifer/jen"

	"tforge/schema"
)

// ResourceGenerator handles generation of resource-specific code.
type ResourceGenerator struct {
	config Config
	file   *jen.File
}

// NewResourceGenerator creates a new resource generator.
func NewResourceGenerator(cfg Config) *ResourceGenerator {
	return &ResourceGenerator{
		config: cfg,
		file:   jen.NewFile(cfg.PackageName),
	}
}

// GenerateResource generates code for a single resource.
func (rg *ResourceGenerator) GenerateResource(res *schema.Resource) error {
	if res == nil {
		return nil
	}

	structName := toGoName(res.Name())

	// Generate main struct
	rg.generateResourceStruct(res, structName)

	// Generate nested types
	rg.generateResourceNestedTypes(res.Block(), structName)

	// Generate constructor
	rg.generateResourceConstructor(res, structName)

	return nil
}

// generateResourceStruct generates the main resource struct.
func (rg *ResourceGenerator) generateResourceStruct(res *schema.Resource, structName string) {
	block := res.Block()
	if block == nil {
		rg.file.Type().Id(structName).Struct()
		return
	}

	if rg.config.GenerateComments && res.Description() != "" {
		rg.file.Comment(wrapComment(res.Description(), 80))
	}

	rg.file.Type().Id(structName).StructFunc(func(s *jen.Group) {
		// Add metadata fields
		s.Comment("Terraform resource metadata")
		s.Id("resourceType").String().Tag(map[string]string{"json": "-"})
		s.Id("resourceName").String().Tag(map[string]string{"json": "-"})
		s.Line()

		// Add schema fields
		rg.generateSchemaFields(s, block)
	})
}

// generateSchemaFields generates struct fields from a schema block.
func (rg *ResourceGenerator) generateSchemaFields(s *jen.Group, block *schema.Block) {
	// Generate attributes first
	attrs := block.Attributes()
	if len(attrs) > 0 {
		s.Comment("Attributes")
	}
	for _, attr := range attrs {
		rg.generateAttributeField(s, attr)
	}

	// Then nested blocks
	nestedBlocks := block.NestedBlocks()
	if len(nestedBlocks) > 0 {
		s.Line()
		s.Comment("Nested blocks")
	}
	for _, nb := range nestedBlocks {
		rg.generateNestedBlockField(s, nb)
	}
}

// generateAttributeField generates a field for an attribute.
func (rg *ResourceGenerator) generateAttributeField(s *jen.Group, attr *schema.Attribute) {
	fieldName := toGoName(attr.Name())
	goType := schema.GoTypeName(attr.Type())
	jsonTag := attr.Name()

	if (attr.Optional() || attr.Computed()) && rg.config.OmitEmpty {
		jsonTag += ",omitempty"
	}

	var field *jen.Statement

	// Add deprecation comment if applicable
	if attr.Deprecated() != "" {
		field = s.Comment("Deprecated: " + attr.Deprecated())
		field = s.Id(fieldName)
	} else {
		field = s.Id(fieldName)
	}

	// Set the type
	field = field.Add(jenTypeFromString(goType))

	// Add tags
	field.Tag(map[string]string{"json": jsonTag})
}

// generateNestedBlockField generates a field for a nested block.
func (rg *ResourceGenerator) generateNestedBlockField(s *jen.Group, nb *schema.NestedBlock) {
	fieldName := toGoName(nb.Name())
	typeName := fieldName + "Block"
	jsonTag := nb.Name() + ",omitempty"

	field := s.Id(fieldName)

	switch nb.NestingMode() {
	case "single", "group":
		field.Op("*").Id(typeName)
	case "list", "set":
		field.Index().Op("*").Id(typeName)
	case "map":
		field.Map(jen.String()).Op("*").Id(typeName)
	default:
		field.Op("*").Id(typeName)
	}

	field.Tag(map[string]string{"json": jsonTag})
}

// generateResourceNestedTypes generates nested block types for a resource.
func (rg *ResourceGenerator) generateResourceNestedTypes(block *schema.Block, parentName string) {
	if block == nil {
		return
	}

	for _, nb := range block.NestedBlocks() {
		typeName := toGoName(nb.Name()) + "Block"

		rg.file.Type().Id(typeName).StructFunc(func(s *jen.Group) {
			innerBlock := nb.Block()
			if innerBlock != nil {
				rg.generateSchemaFields(s, innerBlock)
			}
		})
		rg.file.Line()

		// Recurse for nested blocks within nested blocks
		rg.generateResourceNestedTypes(nb.Block(), typeName)
	}
}

// generateResourceConstructor generates a constructor function for the resource.
func (rg *ResourceGenerator) generateResourceConstructor(res *schema.Resource, structName string) {
	funcName := "New" + structName

	rg.file.Commentf("%s creates a new %s resource.", funcName, res.Name())
	rg.file.Func().Id(funcName).Params(
		jen.Id("name").String(),
	).Op("*").Id(structName).Block(
		jen.Return(jen.Op("&").Id(structName).Values(jen.Dict{
			jen.Id("resourceType"): jen.Lit(res.Name()),
			jen.Id("resourceName"): jen.Id("name"),
		})),
	)
}

// jenTypeFromString converts a Go type string to a jen statement.
func jenTypeFromString(goType string) *jen.Statement {
	switch goType {
	case "string":
		return jen.String()
	case "float64":
		return jen.Float64()
	case "int":
		return jen.Int()
	case "bool":
		return jen.Bool()
	case "any":
		return jen.Any()
	default:
		// Handle slices
		if len(goType) > 2 && goType[:2] == "[]" {
			inner := goType[2:]
			return jen.Index().Add(jenTypeFromString(inner))
		}
		// Handle maps
		if len(goType) > 11 && goType[:11] == "map[string]" {
			inner := goType[11:]
			return jen.Map(jen.String()).Add(jenTypeFromString(inner))
		}
		// Fallback to identifier
		return jen.Id(goType)
	}
}

// File returns the generated jennifer file.
func (rg *ResourceGenerator) File() *jen.File {
	return rg.file
}

