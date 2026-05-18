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

	"github.com/DataDog/tforge/provider"
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

	// Generate helper functions (Ptr, etc.)
	if err := g.generateHelpers(); err != nil {
		return fmt.Errorf("generating helpers: %w", err)
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
//
// Output (example with Datadog provider):
//
//	type ProviderConfig struct {
//	    APIKey  string `json:"api_key"`
//	    AppKey  string `json:"app_key"`
//	    APIUrl  string `json:"api_url,omitempty"`
//	}
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
//
// For each resource (e.g., "datadog_monitor"), it generates:
//  1. A struct type         → type Monitor struct { ... ; resource *config.Resource }
//  2. Nested block types    → type MonitorThresholds struct { ... }
//  3. A functional option   → type MonitorOption func(*Monitor)
//  4. Set* option functions → func Monitor_SetName(v string) MonitorOption { ... }
//  5. A From* bridge        → func FromMonitor(cfg Monitor) MonitorOption { ... }
//  6. A New* constructor    → func NewMonitor(stack, name) (*Monitor, error) { ... }
//  7. Apply / ApplyResourceOptions methods on *Monitor.
func (g *Generator) generateResources() error {
	resources := sortedSchemas(g.schema.ResourceSchemas)
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
			// Unexported back-pointer to the stack-owned Resource.
			// Lowercase → skipped by encoding/json so it never appears in the rendered config.
			s.Id("resource").Op("*").Qual("github.com/DataDog/tforge/config", "Resource")
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

		// Generate typed constructor and Apply* methods.
		g.generateConstructor(f, structName, res.Name, true /* isResource */)
	}

	return g.writeFile(f, "resources_gen.go")
}

// generateDataSources generates data source files.
// Mirrors generateResources but appends "Data" to struct names
// (e.g., "datadog_monitor" data source → MonitorData, MonitorDataOption, NewMonitorData).
func (g *Generator) generateDataSources() error {
	dataSources := sortedSchemas(g.schema.DataSourceSchemas)
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
			// Unexported back-pointer to the stack-owned DataSource.
			s.Id("resource").Op("*").Qual("github.com/DataDog/tforge/config", "DataSource")
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

		// Generate typed constructor and Apply method.
		g.generateConstructor(f, structName, ds.Name, false /* isResource */)
	}

	return g.writeFile(f, "data_sources_gen.go")
}

// generateBlockFields generates struct fields for a block's attributes and nested blocks.
//
// Output (for a block with attributes and nested blocks):
//
//	Name    string                       `json:"name"`
//	Tags    []string                     `json:"tags,omitempty"`
//	Filter  *ApmRetentionFilterFilter    `json:"filter,omitempty"`
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

// needsPointer reports whether an attribute should use a pointer type to
// distinguish zero values from unset. This applies to optional float64 and bool
// fields, where Go's zero value (0, false) would be silently omitted by
// json:",omitempty" even though Terraform treats them as meaningful.
func needsPointer(attr *provider.Attribute, goType string) bool {
	if attr == nil || attr.Required {
		return false
	}
	// Computed-only fields are read-only; never set by the user.
	if attr.Computed && !attr.Optional {
		return false
	}
	// Only float64 and bool have the zero-value ambiguity problem.
	// Strings: empty string omission is correct for Terraform.
	// Slices/maps: nil omission is correct.
	return goType == "float64" || goType == "bool"
}

// applyGoType applies a Go type string to a jen statement. This is the single
// source of truth for mapping Go type names to jennifer type calls, used by both
// struct field generation and option function parameter generation.
//
// Pointer types (e.g. "*float64") are derived from needsPointer — this function
// handles them generically via the "*" prefix so adding a new pointer-eligible
// type only requires updating needsPointer.
func applyGoType(stmt *jen.Statement, goType string) *jen.Statement {
	if strings.HasPrefix(goType, "*") {
		return applyGoType(stmt.Op("*"), goType[1:])
	}
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
//
// Output: Name string `json:"name"` (required) or Tags []string `json:"tags,omitempty"` (optional)
//
// Optional float64 and bool fields use pointer types (*float64, *bool) so that
// nil (unset) is distinguishable from the zero value (0, false) during JSON
// marshaling with omitempty.
func (g *Generator) generateAttributeField(s *jen.Group, attr attributeEntry) {
	fieldName := g.toGoName(attr.Name)
	goType := goTypeName(attributeType(attr.Attr))
	if needsPointer(attr.Attr, goType) {
		goType = "*" + goType
	}
	jsonTag := g.jsonTag(attr.Name, attr.Attr.Optional || attr.Attr.Computed)

	field := s.Id(fieldName)
	applyGoType(field, goType)

	// Add JSON tag
	field.Tag(map[string]string{"json": jsonTag})
}

// generateNestedBlockField generates a struct field for a nested block.
// The type name is prefixed with the parent type name to avoid collisions.
//
// Output varies by nesting mode:
//
//	single/group: Filter  *MonitorFilter            `json:"filter,omitempty"`
//	list/set:     Widget  []*DashboardWidget        `json:"widget,omitempty"`
//	map:          Entries map[string]*ConfigEntries  `json:"entries,omitempty"`
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
//
// Output (for a "filter" block nested inside "ApmRetentionFilter"):
//
//	type ApmRetentionFilterFilter struct {
//	    Query string `json:"query"`
//	}
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

		// Generate functional option type, option functions, and constructor
		g.generateOptionType(f, typeName)
		if nb.Block.Block != nil {
			g.generateOptionFunctions(f, typeName, nb.Block.Block)
		}
		g.generateNestedConstructor(f, typeName)

		// Recursively generate nested types
		if nb.Block.Block != nil {
			g.generateNestedTypes(f, nb.Block.Block, typeName)
		}
	}
}

// generateOptionType generates the functional option type for a resource or data source.
//
// Output: type MonitorOption func(*Monitor)
func (g *Generator) generateOptionType(f *jen.File, structName string) {
	short := g.shortName(structName)
	optionType := short + "Option"
	f.Type().Id(optionType).Func().Params(jen.Op("*").Id(structName))
	f.Line()
}

// emitOptionFunc generates a single Set* option function.
// paramCode is the jen expression for the function parameter — the only thing
// that varies between attribute setters and different nested block nesting modes.
// When takeAddr is true, the closure stores &v instead of v (for pointer fields
// where the parameter accepts the plain type but the struct field is a pointer).
//
// Output (takeAddr=false):
//
//	func Monitor_SetName(v string) MonitorOption {
//	    return func(o *Monitor) { o.Name = v }
//	}
//
// Output (takeAddr=true):
//
//	func Monitor_SetRenotifyInterval(v float64) MonitorOption {
//	    return func(o *Monitor) { o.RenotifyInterval = &v }
//	}
func (g *Generator) emitOptionFunc(f *jen.File, funcName, optionType, structName, fieldName string, paramCode *jen.Statement, takeAddr bool) {
	rhs := jen.Id("v")
	if takeAddr {
		rhs = jen.Op("&").Id("v")
	}
	f.Func().Id(funcName).Params(paramCode).Id(optionType).Block(
		jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
			jen.Id("o").Dot(fieldName).Op("=").Add(rhs),
		)),
	)
	f.Line()
}

// generateOptionFunctions generates a Set* option function for each attribute and nested block.
// Function names use StructName_SetField to avoid ambiguous PascalCase collisions
// (e.g., AwsConnectUser+HierarchyGroupID == AwsConnectUserHierarchyGroup+ID).
//
// Output for attributes:
//
//	func Monitor_SetName(v string) MonitorOption { ... }
//
// Output for nested blocks (parameter varies by nesting mode):
//
//	single/group: func Monitor_SetFilter(v *MonitorFilter) MonitorOption { ... }
//	list/set:     func Dashboard_SetWidget(v ...*DashboardWidget) DashboardOption { ... }
//	map:          func Config_SetEntries(v map[string]*ConfigEntries) ConfigOption { ... }
func (g *Generator) generateOptionFunctions(f *jen.File, structName string, block *provider.BlockType) {
	if block == nil {
		return
	}
	short := g.shortName(structName)
	optionType := short + "Option"

	// Attribute setters: parameter type comes from the Terraform schema type mapping.
	// For pointer fields (*float64, *bool), the option function accepts the plain
	// value type and takes its address inside the closure, keeping the user API clean.
	for _, attr := range sortedAttributes(block) {
		fieldName := g.toGoName(attr.Name)
		funcName := structName + "_Set" + fieldName
		goType := goTypeName(attributeType(attr.Attr))
		isPtr := needsPointer(attr.Attr, goType)

		// Option function parameter is always the plain type (never pointer).
		param := applyGoType(jen.Id("v"), goType)
		g.emitOptionFunc(f, funcName, optionType, structName, fieldName, param, isPtr)
	}

	// Nested block setters: parameter shape depends on nesting mode.
	for _, nb := range sortedNestedBlocks(block) {
		fieldName := g.toGoName(nb.Name)
		typeName := structName + g.toGoName(nb.Name)
		funcName := structName + "_Set" + fieldName

		var param *jen.Statement
		switch nb.Block.NestingMode {
		case provider.NestingModeSingle, provider.NestingModeGroup:
			param = jen.Id("v").Op("*").Id(typeName)
		case provider.NestingModeList, provider.NestingModeSet:
			param = jen.Id("v").Op("...").Op("*").Id(typeName)
		case provider.NestingModeMap:
			param = jen.Id("v").Map(jen.String()).Op("*").Id(typeName)
		default:
			param = jen.Id("v").Op("*").Id(typeName)
		}
		g.emitOptionFunc(f, funcName, optionType, structName, fieldName, param, false)
	}
}

// generateConfigBridge generates the FromXxx bridge function for struct-based migration.
// The "From" prefix avoids collisions with field option functions (which use "Set").
// The closure preserves the unexported `resource` back-pointer across the
// struct assignment so the handle stays attached to the stack:
//
//	func FromMonitor(cfg Monitor) MonitorOption {
//	    return func(o *Monitor) {
//	        save := o.resource
//	        *o = cfg
//	        o.resource = save
//	    }
//	}
func (g *Generator) generateConfigBridge(f *jen.File, structName string) {
	short := g.shortName(structName)
	optionType := short + "Option"
	funcName := "From" + short

	f.Func().Id(funcName).Params(
		jen.Id("cfg").Id(structName),
	).Id(optionType).Block(
		jen.Return(jen.Func().Params(jen.Id("o").Op("*").Id(structName)).Block(
			jen.Id("save").Op(":=").Id("o").Dot("resource"),
			jen.Op("*").Id("o").Op("=").Id("cfg"),
			jen.Id("o").Dot("resource").Op("=").Id("save"),
		)),
	)
	f.Line()
}

// generateConstructor generates a typed constructor + Apply method(s) for a
// top-level resource or data source.
//
// For resources (isResource=true), it emits:
//
//	func NewMonitor(stack *config.Stack, name string) (*Monitor, error) {
//	    m := &Monitor{}
//	    r := &config.Resource{Type: "datadog_monitor", Name: name, TypedAttributes: m}
//	    if err := stack.AddResourcePtr(r); err != nil { return nil, err }
//	    m.resource = r
//	    return m, nil
//	}
//	func (m *Monitor) Apply(opts ...MonitorOption) *Monitor { ... }
//	func (m *Monitor) ApplyResourceOptions(opts ...config.ResourceOption) { ... }
//
// For data sources (isResource=false), it emits the same New + Apply pair
// minus ApplyResourceOptions (DataSource has no meta-option helpers today).
func (g *Generator) generateConstructor(f *jen.File, structName, tfTypeName string, isResource bool) {
	funcName := "New" + structName
	short := g.shortName(structName)
	optionType := short + "Option"

	configType := "Resource"
	addMethod := "AddResourcePtr"
	kind := "resource"
	if !isResource {
		configType = "DataSource"
		addMethod = "AddDataSourcePtr"
		kind = "data source"
	}

	// Constructor: build the handle, attach the typed payload, add to stack.
	f.Commentf("%s creates a %s %s, attaches it to the stack, and returns a", funcName, tfTypeName, kind)
	f.Commentf("handle whose Apply method configures the %s's attributes.", kind)
	f.Func().Id(funcName).Params(
		jen.Id("stack").Op("*").Qual("github.com/DataDog/tforge/config", "Stack"),
		jen.Id("name").String(),
	).Params(jen.Op("*").Id(structName), jen.Error()).Block(
		jen.Id("h").Op(":=").Op("&").Id(structName).Values(),
		jen.Id("r").Op(":=").Op("&").Qual("github.com/DataDog/tforge/config", configType).Values(jen.Dict{
			jen.Id("Type"):            jen.Lit(tfTypeName),
			jen.Id("Name"):            jen.Id("name"),
			jen.Id("TypedAttributes"): jen.Id("h"),
		}),
		jen.If(
			jen.Id("err").Op(":=").Id("stack").Dot(addMethod).Call(jen.Id("r")),
			jen.Id("err").Op("!=").Nil(),
		).Block(
			jen.Return(jen.Nil(), jen.Id("err")),
		),
		jen.Id("h").Dot("resource").Op("=").Id("r"),
		jen.Return(jen.Id("h"), jen.Nil()),
	)
	f.Line()

	// Apply method.
	f.Commentf("Apply sets attribute fields on the %s. Safe to call multiple times;", tfTypeName)
	f.Comment("each call mutates the typed payload in place. Returns the receiver for chaining.")
	f.Func().Params(jen.Id("h").Op("*").Id(structName)).Id("Apply").Params(
		jen.Id("opts").Op("...").Id(optionType),
	).Op("*").Id(structName).Block(
		jen.For(jen.List(jen.Id("_"), jen.Id("opt")).Op(":=").Range().Id("opts")).Block(
			jen.Id("opt").Call(jen.Id("h")),
		),
		jen.Return(jen.Id("h")),
	)
	f.Line()

	if isResource {
		// ApplyResourceOptions method (meta-arguments).
		// The nil guard makes the method a no-op when the handle was
		// constructed outside New<T> (e.g., &<T>{}), avoiding a confusing
		// nil-deref panic. Direct stack mutation via Apply is unaffected.
		f.Commentf("ApplyResourceOptions sets meta-arguments (Lifecycle, Provider, DependsOn,")
		f.Comment("Timeouts) on the underlying resource. Safe to call multiple times.")
		f.Comment("Has no effect if the handle was not created by the New constructor.")
		f.Func().Params(jen.Id("h").Op("*").Id(structName)).Id("ApplyResourceOptions").Params(
			jen.Id("opts").Op("...").Qual("github.com/DataDog/tforge/config", "ResourceOption"),
		).Block(
			jen.If(jen.Id("h").Dot("resource").Op("==").Nil()).Block(jen.Return()),
			jen.For(jen.List(jen.Id("_"), jen.Id("opt")).Op(":=").Range().Id("opts")).Block(
				jen.Id("opt").Call(jen.Id("h").Dot("resource")),
			),
		)
		f.Line()
	}
}

// generateNestedConstructor generates a constructor for a nested block type.
// Unlike top-level constructors (which add to a Stack), nested constructors
// simply apply options and return a pointer to the configured struct.
//
// Output:
//
//	func NewContainerPorts(opts ...ContainerPortsOption) *ContainerPorts {
//	    var cfg ContainerPorts
//	    for _, opt := range opts {
//	        opt(&cfg)
//	    }
//	    return &cfg
//	}
func (g *Generator) generateNestedConstructor(f *jen.File, structName string) {
	short := g.shortName(structName)
	optionType := short + "Option"
	funcName := "New" + short

	f.Commentf("%s creates a new %s from functional options.", funcName, structName)
	f.Func().Id(funcName).Params(
		jen.Id("opts").Op("...").Id(optionType),
	).Op("*").Id(structName).Block(
		jen.Var().Id("cfg").Id(structName),
		jen.For(jen.List(jen.Id("_"), jen.Id("opt")).Op(":=").Range().Id("opts")).Block(
			jen.Id("opt").Call(jen.Op("&").Id("cfg")),
		),
		jen.Return(jen.Op("&").Id("cfg")),
	)
	f.Line()
}

// generateHelpers generates utility functions used by generated code.
// Currently emits a generic Ptr[T] function for struct-literal users who need
// to set optional pointer fields without a temporary variable.
func (g *Generator) generateHelpers() error {
	f := jen.NewFile(g.config.PackageName)
	f.HeaderComment("Code generated by tforge. DO NOT EDIT.")

	f.Comment("Ptr returns a pointer to the given value.")
	f.Comment("Use this for optional pointer fields in struct literals:")
	f.Comment("")
	f.Comment("    cfg := Monitor{RenotifyInterval: Ptr(float64(60))}")
	f.Func().Id("Ptr").Types(jen.Id("T").Any()).Params(
		jen.Id("v").Id("T"),
	).Op("*").Id("T").Block(
		jen.Return(jen.Op("&").Id("v")),
	)
	f.Line()

	return g.writeFile(f, "helpers_gen.go")
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
	outDir, err := filepath.Abs(g.config.OutputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}
	path := filepath.Join(outDir, filename)
	if !strings.HasPrefix(path, outDir+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes output directory %q", filename, g.config.OutputDir)
	}
	return WriteFile(f, path)
}
