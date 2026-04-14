// Package generate provides code generation from Terraform provider schemas.
//
// This package generates idiomatic Go SDK code from provider schemas, including:
//   - Provider configuration structs
//   - Resource structs with JSON tags
//   - Data source structs
//   - Nested block types
//
// The generated code is designed to work with the config and render packages.
//
// Example usage:
//
//	gen := generate.New(generate.Config{
//	    OutputDir:   "./generated/datadog",
//	    PackageName: "datadog",
//	})
//	if err := gen.Generate(schema); err != nil {
//	    log.Fatal(err)
//	}
//
// Documentation comments and omitempty tags for optional fields are always generated.
package generate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"

	"tforge/provider"
)

// Config holds configuration for the code generator.
type Config struct {
	// OutputDir is the directory where generated files will be written.
	OutputDir string

	// PackageName is the Go package name for generated code.
	PackageName string

	// Initialisms is the set of terms that should remain fully uppercased
	// in generated Go identifiers (e.g. "ID" → ID, "API" → API).
	//
	// When set, this fully overrides the default list — only the entries
	// you provide will be treated as initialisms.
	//
	// When nil, DefaultInitialisms is used.
	Initialisms []string
}

// Generator generates Go code from provider schemas.
type Generator struct {
	config         Config
	schema         *provider.ProviderSchema
	reservedNames  map[string]bool // top-level type names to avoid collisions
	initialisms    map[string]bool // resolved initialism set for identifier conversion
	providerPrefix string          // Go-ified provider prefix (e.g. "Datadog" for datadog_*)
}

const (
	// DefaultPackageName is the default Go package name for generated code
	// when no explicit name is provided.
	DefaultPackageName = "generated"

	// DefaultOutputDir is the default output directory for generated code
	// when no explicit directory is provided.
	DefaultOutputDir = "./generated"
)

// New creates a new Generator with the given configuration.
func New(cfg Config) *Generator {
	if cfg.PackageName == "" {
		cfg.PackageName = DefaultPackageName
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = DefaultOutputDir
	}

	initialisms := make(map[string]bool)
	src := cfg.Initialisms
	if src == nil {
		src = DefaultInitialisms
	}
	for _, s := range src {
		initialisms[strings.ToUpper(s)] = true
	}

	return &Generator{config: cfg, initialisms: initialisms}
}

// Generate generates Go SDK code from a provider schema.
func (g *Generator) Generate(ps *provider.ProviderSchema) error {
	if ps == nil {
		return fmt.Errorf("nil provider schema")
	}

	g.schema = ps

	// Derive the provider prefix from resource/data source names.
	// All resources in a provider share the same first _-delimited segment
	// (e.g., "datadog_monitor" → "datadog" → "Datadog").
	g.providerPrefix = g.detectProviderPrefix()

	// Collect all top-level type names so nested block types can avoid collisions.
	// E.g., aws_s3_bucket has nested block "logging" → S3BucketLogging,
	// but aws_s3_bucket_logging is also a standalone resource → S3BucketLogging.
	g.reservedNames = make(map[string]bool)
	for name := range g.schema.ResourceSchemas {
		structName := g.shortName(g.toGoName(name))
		g.reservedNames[structName] = true
		g.reservedNames[structName+"Option"] = true
	}
	for name := range g.schema.DataSourceSchemas {
		structName := g.shortName(g.toGoName(name)) + "Data"
		g.reservedNames[structName] = true
		g.reservedNames[structName+"Option"] = true
	}

	// Generate provider configuration
	if err := g.generateProvider(); err != nil {
		return fmt.Errorf("generating provider: %w", err)
	}

	// Generate resources
	if err := g.generateResources(); err != nil {
		return fmt.Errorf("generating resources: %w", err)
	}

	// Generate data sources
	if err := g.generateDataSources(); err != nil {
		return fmt.Errorf("generating data sources: %w", err)
	}

	return nil
}

// detectProviderPrefix extracts the Go-ified provider prefix from resource names.
// For "datadog_monitor", "datadog_slo" → prefix is "Datadog".
func (g *Generator) detectProviderPrefix() string {
	// Try resources first, then data sources.
	for name := range g.schema.ResourceSchemas {
		if idx := strings.IndexByte(name, '_'); idx > 0 {
			return g.toGoName(name[:idx])
		}
	}
	for name := range g.schema.DataSourceSchemas {
		if idx := strings.IndexByte(name, '_'); idx > 0 {
			return g.toGoName(name[:idx])
		}
	}
	return ""
}

// shortName strips the provider prefix from a Go struct name.
// "DatadogMonitor" with prefix "Datadog" → "Monitor".
func (g *Generator) shortName(structName string) string {
	if g.providerPrefix != "" && strings.HasPrefix(structName, g.providerPrefix) {
		short := strings.TrimPrefix(structName, g.providerPrefix)
		if short != "" {
			return short
		}
	}
	return structName
}

// generateProvider generates the provider configuration file.
func (g *Generator) generateProvider() error {
	f := jen.NewFile(g.config.PackageName)
	f.HeaderComment("Code generated by tforge. DO NOT EDIT.")

	var block *provider.BlockType
	if g.schema.ConfigSchema != nil {
		block = g.schema.ConfigSchema.Block
	}

	if block == nil {
		// Empty provider config
		f.Comment("ProviderConfig represents the provider configuration.")
		f.Type().Id("ProviderConfig").Struct()
	} else {
		if block.Description != "" {
			f.Comment(block.Description)
		}
		f.Comment("ProviderConfig represents the provider configuration.")
		f.Type().Id("ProviderConfig").StructFunc(func(s *jen.Group) {
			g.generateBlockFields(s, block, "ProviderConfig")
		})

		// Generate nested block types for provider config
		g.generateNestedTypes(f, block, "ProviderConfig")
	}

	return g.writeFile(f, "provider_gen.go")
}

// generateResources generates resource files.
func (g *Generator) generateResources() error {
	resources := sortedResources(g.schema.ResourceSchemas)
	if len(resources) == 0 {
		return nil
	}

	f := jen.NewFile(g.config.PackageName)
	f.HeaderComment("Code generated by tforge. DO NOT EDIT.")

	for _, res := range resources {
		structName := g.shortName(g.toGoName(res.Name))

		// Type name comment first (Go convention), then description
		f.Commentf("%s represents the %s resource.", structName, res.Name)
		if res.Schema != nil && res.Schema.Block != nil && res.Schema.Block.Description != "" {
			f.Comment(cleanDescription(res.Schema.Block.Description))
		}
		f.Type().Id(structName).StructFunc(func(s *jen.Group) {
			if res.Schema != nil && res.Schema.Block != nil {
				g.generateBlockFields(s, res.Schema.Block, structName)
			}
		})
		f.Line()

		// Generate nested block types for this resource
		if res.Schema != nil && res.Schema.Block != nil {
			g.generateNestedTypes(f, res.Schema.Block, structName)
		}

		// Generate functional option type, option functions, and config bridge
		g.generateOptionType(f, structName)
		if res.Schema != nil && res.Schema.Block != nil {
			g.generateOptionFunctions(f, structName, res.Schema.Block)
		}
		g.generateConfigBridge(f, structName)

		// Generate typed constructor: NewXxx(stack, name, opts ...XxxOption) error
		g.generateConstructor(f, structName, res.Name, "AddResource")
	}

	return g.writeFile(f, "resources_gen.go")
}

// generateDataSources generates data source files.
func (g *Generator) generateDataSources() error {
	dataSources := sortedDataSources(g.schema.DataSourceSchemas)
	if len(dataSources) == 0 {
		return nil
	}

	f := jen.NewFile(g.config.PackageName)
	f.HeaderComment("Code generated by tforge. DO NOT EDIT.")

	for _, ds := range dataSources {
		structName := g.shortName(g.toGoName(ds.Name)) + "Data"

		// Type name comment first (Go convention), then description
		f.Commentf("%s represents the %s data source.", structName, ds.Name)
		if ds.Schema != nil && ds.Schema.Block != nil && ds.Schema.Block.Description != "" {
			f.Comment(cleanDescription(ds.Schema.Block.Description))
		}
		f.Type().Id(structName).StructFunc(func(s *jen.Group) {
			if ds.Schema != nil && ds.Schema.Block != nil {
				g.generateBlockFields(s, ds.Schema.Block, structName)
			}
		})
		f.Line()

		// Generate nested block types for this data source
		if ds.Schema != nil && ds.Schema.Block != nil {
			g.generateNestedTypes(f, ds.Schema.Block, structName)
		}

		// Generate functional option type, option functions, and config bridge
		g.generateOptionType(f, structName)
		if ds.Schema != nil && ds.Schema.Block != nil {
			g.generateOptionFunctions(f, structName, ds.Schema.Block)
		}
		g.generateConfigBridge(f, structName)

		// Generate typed constructor: NewXxx(stack, name, opts ...XxxOption) error
		g.generateConstructor(f, structName, ds.Name, "AddDataSource")
	}

	return g.writeFile(f, "data_sources_gen.go")
}

// generateBlockFields generates struct fields for a block's attributes and nested blocks.
func (g *Generator) generateBlockFields(s *jen.Group, block *provider.BlockType, parentTypeName string) {
	// Generate attributes
	for _, attr := range sortedAttributes(block) {
		g.generateAttributeField(s, attr)
	}

	// Generate nested blocks
	for _, nb := range sortedNestedBlocks(block) {
		g.generateNestedBlockField(s, nb, parentTypeName)
	}
}

// applyGoType applies a Go type string to a jen statement. This is the single
// source of truth for mapping Go type names to jennifer type calls, used by both
// struct field generation and option function parameter generation.
func applyGoType(stmt *jen.Statement, goType string) *jen.Statement {
	switch goType {
	case "string":
		return stmt.String()
	case "float64":
		return stmt.Float64()
	case "bool":
		return stmt.Bool()
	case "any":
		return stmt.Any()
	default:
		return stmt.Id(goType)
	}
}

// generateAttributeField generates a struct field for an attribute.
func (g *Generator) generateAttributeField(s *jen.Group, attr attributeEntry) {
	fieldName := g.toGoName(attr.Name)
	goType := goTypeName(attributeType(attr.Attr))
	jsonTag := g.jsonTag(attr.Name, attr.Attr.Optional || attr.Attr.Computed)

	field := s.Id(fieldName)
	applyGoType(field, goType)

	// Add JSON tag
	field.Tag(map[string]string{"json": jsonTag})
}

// generateNestedBlockField generates a struct field for a nested block.
// The type name is prefixed with the parent type name to avoid collisions.
func (g *Generator) generateNestedBlockField(s *jen.Group, nb nestedBlockEntry, parentTypeName string) {
	fieldName := g.toGoName(nb.Name)
	// Prefix with parent type name to make unique
	typeName := parentTypeName + g.toGoName(nb.Name)
	jsonTag := g.jsonTag(nb.Name, true)

	field := s.Id(fieldName)

	switch nb.Block.NestingMode {
	case provider.NestingModeSingle, provider.NestingModeGroup:
		field.Op("*").Id(typeName)
	case provider.NestingModeList, provider.NestingModeSet:
		field.Index().Op("*").Id(typeName)
	case provider.NestingModeMap:
		field.Map(jen.String()).Op("*").Id(typeName)
	default:
		field.Op("*").Id(typeName)
	}

	field.Tag(map[string]string{"json": jsonTag})
}

// generateNestedTypes generates struct types for nested blocks.
// Type names are prefixed with the parent type name to avoid collisions.
func (g *Generator) generateNestedTypes(f *jen.File, block *provider.BlockType, parentTypeName string) {
	if block == nil {
		return
	}

	for _, nb := range sortedNestedBlocks(block) {
		// Prefix with parent type name to make unique
		typeName := parentTypeName + g.toGoName(nb.Name)

		// Skip if this name collides with a top-level resource or data source.
		// The standalone resource type takes precedence.
		if g.reservedNames[typeName] {
			continue
		}

		// Type name comment first (Go convention), then description
		f.Commentf("%s represents a %s block in %s.", typeName, nb.Name, parentTypeName)
		if nb.Block.Block != nil && nb.Block.Block.Description != "" {
			f.Comment(cleanDescription(nb.Block.Block.Description))
		}
		f.Type().Id(typeName).StructFunc(func(s *jen.Group) {
			if nb.Block.Block != nil {
				g.generateBlockFields(s, nb.Block.Block, typeName)
			}
		})
		f.Line()

		// Recursively generate nested types
		if nb.Block.Block != nil {
			g.generateNestedTypes(f, nb.Block.Block, typeName)
		}
	}
}

// generateOptionType generates the functional option type for a resource or data source.
// Example output: type MonitorOption func(*DatadogMonitor)
func (g *Generator) generateOptionType(f *jen.File, structName string) {
	short := g.shortName(structName)
	optionType := short + "Option"
	f.Commentf("%s is a functional option for configuring a %s.", optionType, structName)
	f.Type().Id(optionType).Func().Params(jen.Op("*").Id(structName))
	f.Line()
}

// generateOptionFunctions generates a Set* option function for each attribute and nested block.
// Option function names use the full struct name with a "Set" prefix and underscore separator
// (e.g., DatadogMonitor_SetName) to avoid ambiguous PascalCase concatenation collisions
// (e.g., AwsConnectUser+HierarchyGroupID == AwsConnectUserHierarchyGroup+ID).
func (g *Generator) generateOptionFunctions(f *jen.File, structName string, block *provider.BlockType) {
	if block == nil {
		return
	}
	short := g.shortName(structName)
	optionType := short + "Option"

	for _, attr := range sortedAttributes(block) {
		fieldName := g.toGoName(attr.Name)
		funcName := structName + "_Set" + fieldName
		goType := goTypeName(attributeType(attr.Attr))

		f.Commentf("%s sets the %s field.", funcName, fieldName)
		f.Func().Id(funcName).ParamsFunc(func(p *jen.Group) {
			applyGoType(p.Id("v"), goType)
		}).Id(optionType).Block(
			jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
				jen.Id("o").Dot(fieldName).Op("=").Id("v"),
			)),
		)
		f.Line()
	}

	for _, nb := range sortedNestedBlocks(block) {
		fieldName := g.toGoName(nb.Name)
		typeName := structName + g.toGoName(nb.Name)
		funcName := structName + "_Set" + fieldName

		f.Commentf("%s sets the %s nested block.", funcName, fieldName)
		switch nb.Block.NestingMode {
		case provider.NestingModeSingle, provider.NestingModeGroup:
			f.Func().Id(funcName).Params(
				jen.Id("v").Op("*").Id(typeName),
			).Id(optionType).Block(
				jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
					jen.Id("o").Dot(fieldName).Op("=").Id("v"),
				)),
			)
		case provider.NestingModeList, provider.NestingModeSet:
			f.Func().Id(funcName).Params(
				jen.Id("v").Op("...").Op("*").Id(typeName),
			).Id(optionType).Block(
				jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
					jen.Id("o").Dot(fieldName).Op("=").Id("v"),
				)),
			)
		case provider.NestingModeMap:
			f.Func().Id(funcName).Params(
				jen.Id("v").Map(jen.String()).Op("*").Id(typeName),
			).Id(optionType).Block(
				jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
					jen.Id("o").Dot(fieldName).Op("=").Id("v"),
				)),
			)
		default:
			f.Func().Id(funcName).Params(
				jen.Id("v").Op("*").Id(typeName),
			).Id(optionType).Block(
				jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
					jen.Id("o").Dot(fieldName).Op("=").Id("v"),
				)),
			)
		}
		f.Line()
	}
}

// generateConfigBridge generates the FromXxx bridge function for struct-based migration.
// Example output: func FromMonitor(cfg DatadogMonitor) MonitorOption { ... }
// The "From" prefix avoids collisions with field option functions (which use "With").
func (g *Generator) generateConfigBridge(f *jen.File, structName string) {
	short := g.shortName(structName)
	optionType := short + "Option"
	funcName := "From" + short

	f.Commentf("%s applies all fields from a %s struct value.", funcName, structName)
	f.Func().Id(funcName).Params(
		jen.Id("cfg").Id(structName),
	).Id(optionType).Block(
		jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
			jen.Op("*").Id("o").Op("=").Id("cfg"),
		)),
	)
	f.Line()
}

// generateConstructor generates a typed constructor function for a resource or data source.
// Example output: func NewAwsInstance(stack *config.Stack, name string, opts ...InstanceOption) error { ... }
func (g *Generator) generateConstructor(f *jen.File, structName, tfTypeName, addMethod string) {
	funcName := "New" + structName
	short := g.shortName(structName)
	optionType := short + "Option"

	// Determine the config type name based on whether this is a resource or data source
	configType := "Resource"
	if addMethod == "AddDataSource" {
		configType = "DataSource"
	}

	f.Commentf("%s creates a %s %s and adds it to the stack.", funcName, tfTypeName, configType)
	f.Func().Id(funcName).Params(
		jen.Id("stack").Op("*").Qual("tforge/config", "Stack"),
		jen.Id("name").String(),
		jen.Id("opts").Op("...").Id(optionType),
	).Error().Block(
		jen.Var().Id("cfg").Id(structName),
		jen.For(jen.List(jen.Id("_"), jen.Id("opt")).Op(":=").Range().Id("opts")).Block(
			jen.Id("opt").Call(jen.Op("&").Id("cfg")),
		),
		jen.List(jen.Id("data"), jen.Id("err")).Op(":=").Qual("encoding/json", "Marshal").Call(jen.Id("cfg")),
		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("marshaling "+tfTypeName+" config: %w"), jen.Id("err"))),
		),
		jen.Var().Id("m").Map(jen.String()).Any(),
		jen.If(
			jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(jen.Id("data"), jen.Op("&").Id("m")),
			jen.Id("err").Op("!=").Nil(),
		).Block(
			jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("converting "+tfTypeName+" config: %w"), jen.Id("err"))),
		),
		jen.Return(jen.Id("stack").Dot(addMethod).Call(
			jen.Qual("tforge/config", configType).Values(jen.Dict{
				jen.Id("Type"):   jen.Lit(tfTypeName),
				jen.Id("Name"):   jen.Id("name"),
				jen.Id("Config"): jen.Id("m"),
			}),
		)),
	)
	f.Line()
}

// jsonTag generates a JSON struct tag, adding omitempty for optional fields.
func (g *Generator) jsonTag(name string, optional bool) string {
	if optional {
		return name + ",omitempty"
	}
	return name
}

// writeFile writes a jennifer file to disk.
func (g *Generator) writeFile(f *jen.File, filename string) error {
	path := filepath.Join(g.config.OutputDir, filename)
	return WriteFile(f, path)
}
