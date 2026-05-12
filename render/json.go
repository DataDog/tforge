// Package render converts config.Stack to Terraform JSON format.
//
// This package is responsible for serializing Terraform configurations
// to the .tf.json format that Terraform can read directly.
//
// Example usage:
//
//	stack := config.NewStack("my-infra")
//	// ... add providers, resources, etc.
//
//	// Write to file
//	if err := render.ToFile(stack, "main.tf.json"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Or get JSON bytes
//	data, err := render.ToJSON(stack)
//
// The tforge version stamped into the JSON metadata comes from the
// tforge.Version variable — set it at link time via
// `-ldflags '-X github.com/DataDog/tforge.Version=…'`.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DataDog/tforge"
	"github.com/DataDog/tforge/config"
)

const (
	dirPerm  = os.FileMode(0755)
	filePerm = os.FileMode(0644)
)

// marshalNoEscape marshals v to JSON without HTML-escaping special characters
// like <, >, and &. This matches the behavior of the top-level encoder which
// uses SetEscapeHTML(false), ensuring that values containing these characters
// (e.g., Terraform query expressions with ">") are preserved as-is.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode appends a trailing newline; strip it for MarshalJSON compatibility.
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}

// ToJSON renders a stack to JSON bytes. The tforge version stamped into the
// metadata comment block comes from tforge.Version — see that variable's doc
// for how to override it at link time.
func ToJSON(stack *config.Stack) ([]byte, error) {
	if stack == nil {
		return nil, fmt.Errorf("nil stack")
	}

	tf, err := buildTerraformJSON(stack, tforge.Version)
	if err != nil {
		return nil, err
	}

	// Use deterministic JSON output
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(tf); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}

	return buf.Bytes(), nil
}

// ToFile renders a stack to a JSON file.
func ToFile(stack *config.Stack, path string) error {
	data, err := ToJSON(stack)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// terraformJSON represents the root structure of a .tf.json file.
type terraformJSON struct {
	Comment   map[string]any            `json:"//,omitempty"`
	Terraform *terraformBlock           `json:"terraform,omitempty"`
	Provider  map[string][]any          `json:"provider,omitempty"`
	Resource  map[string]map[string]any `json:"resource,omitempty"`
	Data      map[string]map[string]any `json:"data,omitempty"`
	Module    map[string]any            `json:"module,omitempty"`
	Variable  map[string]variableJSON   `json:"variable,omitempty"`
	Output    map[string]outputJSON     `json:"output,omitempty"`
	Locals    map[string]any            `json:"locals,omitempty"`
}

// terraformBlock represents the terraform {} block.
type terraformBlock struct {
	RequiredVersion   string                          `json:"required_version,omitempty"`
	RequiredProviders map[string]requiredProviderJSON `json:"required_providers,omitempty"`
	Backend           map[string]map[string]any       `json:"backend,omitempty"`
}

// requiredProviderJSON represents a required provider entry.
type requiredProviderJSON struct {
	Source  string `json:"source"`
	Version string `json:"version"`
}

// variableJSON represents a Terraform variable definition.
type variableJSON struct {
	Type        string                     `json:"type,omitempty"`
	Description string                     `json:"description,omitempty"`
	Default     any                        `json:"default,omitempty"`
	Sensitive   bool                       `json:"sensitive,omitempty"`
	Nullable    bool                       `json:"nullable,omitempty"`
	Validation  *config.VariableValidation `json:"validation,omitempty"`
}

// outputJSON represents a Terraform output definition.
type outputJSON struct {
	Value        any               `json:"value"`
	Description  string            `json:"description,omitempty"`
	Sensitive    bool              `json:"sensitive,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Precondition *config.Condition `json:"precondition,omitempty"`
}

// buildTerraformJSON converts a Stack to the terraform JSON structure.
func buildTerraformJSON(stack *config.Stack, version string) (*terraformJSON, error) {
	tf := &terraformJSON{
		Comment: map[string]any{
			"metadata": map[string]any{
				"tool":      "tforge",
				"version":   version,
				"stackName": stack.Name,
			},
		},
	}

	// Build terraform block
	if needsTerraformBlock(stack) {
		tf.Terraform = buildTerraformBlock(stack)
	}

	// Build providers
	if len(stack.Providers) > 0 {
		tf.Provider = buildProviders(stack.Providers)
	}

	// Build resources
	if len(stack.Resources) > 0 {
		res, err := buildResources(stack.Name, stack.Resources)
		if err != nil {
			return nil, err
		}
		tf.Resource = res
	}

	// Build data sources
	if len(stack.DataSources) > 0 {
		data, err := buildDataSources(stack.Name, stack.DataSources)
		if err != nil {
			return nil, err
		}
		tf.Data = data
	}

	// Build modules
	if len(stack.Modules) > 0 {
		tf.Module = buildModules(stack.Name, stack.Modules)
	}

	// Build variables
	if len(stack.Variables) > 0 {
		tf.Variable = buildVariables(stack.Variables)
	}

	// Build outputs
	if len(stack.Outputs) > 0 {
		tf.Output = buildOutputs(stack.Outputs)
	}

	// Build locals
	if len(stack.Locals) > 0 {
		tf.Locals = stack.Locals
	}

	return tf, nil
}

// needsTerraformBlock returns true if the terraform block is needed.
func needsTerraformBlock(stack *config.Stack) bool {
	return len(stack.RequiredProviders) > 0 || stack.Backend != nil
}

// buildTerraformBlock builds the terraform {} block.
func buildTerraformBlock(stack *config.Stack) *terraformBlock {
	tb := &terraformBlock{}

	// Required providers
	if len(stack.RequiredProviders) > 0 {
		tb.RequiredProviders = make(map[string]requiredProviderJSON)
		for name, req := range stack.RequiredProviders {
			tb.RequiredProviders[name] = requiredProviderJSON{
				Source:  req.Source,
				Version: req.Version,
			}
		}
	}

	// Backend
	if stack.Backend != nil {
		tb.Backend = map[string]map[string]any{
			stack.Backend.Type: stack.Backend.Config,
		}
	}

	return tb
}

// buildProviders builds the provider configurations.
func buildProviders(providers []config.Provider) map[string][]any {
	result := make(map[string][]any)

	for _, p := range providers {
		cfg := providerConfigJSON{
			Config: p.Config,
			Alias:  p.Alias,
		}
		result[p.Name] = append(result[p.Name], cfg)
	}

	return result
}

// buildResources builds the resource configurations. stackName is threaded
// through so each resource's "//.metadata.path" can be set to match cdktf's
// per-resource metadata layout: `<stackName>/<pathified-original>` (forward
// slashes replaced with `--` but `{}`, `:`, etc. preserved — the same shape
// cdktf produced via construct-tree node paths). `uniqueId` is the
// SanitizeName'd identifier used as the Terraform resource key.
func buildResources(stackName string, resources []config.Resourcer) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any)

	for _, rr := range resources {
		r := rr.AsResource()
		if result[r.Type] == nil {
			result[r.Type] = make(map[string]any)
		}

		cfg, err := buildResourceConfig(r)
		if err != nil {
			return nil, err
		}
		cfg.Path = stackName + "/" + config.PathifyName(r.Name)
		cfg.UniqueID = r.SanitizedName()
		result[r.Type][r.SanitizedName()] = cfg
	}

	return result, nil
}

// resourceConfigJSON represents a single resource block in JSON output.
// The Config map is flattened alongside the typed meta-argument fields
// via a custom MarshalJSON, giving compile-time safety for the meta keys
// while preserving the arbitrary user-supplied configuration.
type resourceConfigJSON struct {
	Path         string            `json:"-"` // <stackName>/<name>; emitted under "//"
	UniqueID     string            `json:"-"` // <name>; emitted under "//"
	Config       map[string]any    `json:"-"` // flattened into top level
	Provider     string            `json:"provider,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Count        any               `json:"count,omitempty"`
	ForEach      any               `json:"for_each,omitempty"`
	Lifecycle    *config.Lifecycle `json:"lifecycle,omitempty"`
	Timeouts     *config.Timeouts  `json:"timeouts,omitempty"`
	Connection   *connectionJSON   `json:"connection,omitempty"`
	Provisioners []provisionerJSON `json:"provisioner,omitempty"`
}

// MarshalJSON flattens Config alongside the typed meta-argument fields.
// A leading "//" metadata block carrying the resource's path and uniqueId is
// emitted first when Path/UniqueID are set — matching cdktf's per-resource
// metadata layout so existing consumers can parse tforge output unchanged.
func (r resourceConfigJSON) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(r.Config)+9)
	if md := buildResourceMetadata(r.Path, r.UniqueID); md != nil {
		m["//"] = md
	}
	for k, v := range r.Config {
		m[k] = v
	}
	if r.Provider != "" {
		m["provider"] = r.Provider
	}
	if len(r.DependsOn) > 0 {
		m["depends_on"] = r.DependsOn
	}
	if r.Count != nil {
		m["count"] = r.Count
	}
	if r.ForEach != nil {
		m["for_each"] = r.ForEach
	}
	if r.Lifecycle != nil {
		m["lifecycle"] = r.Lifecycle
	}
	if r.Timeouts != nil {
		m["timeouts"] = r.Timeouts
	}
	if r.Connection != nil {
		m["connection"] = r.Connection
	}
	if len(r.Provisioners) > 0 {
		m["provisioner"] = r.Provisioners
	}
	return marshalNoEscape(m)
}

// buildResourceMetadata returns the per-resource "//.metadata" block matching
// cdktf's layout. Returns nil when neither field is set so the "//" key is
// omitted entirely.
func buildResourceMetadata(path, uniqueID string) map[string]any {
	if path == "" && uniqueID == "" {
		return nil
	}
	meta := map[string]any{}
	if path != "" {
		meta["path"] = path
	}
	if uniqueID != "" {
		meta["uniqueId"] = uniqueID
	}
	return map[string]any{"metadata": meta}
}

// provisionerJSON represents a single provisioner block in JSON output.
// Config is flattened alongside the typed meta-keys (when, on_failure,
// connection) to prevent string-key typos.
type provisionerJSON struct {
	Type       string          `json:"-"` // used as the outer map key
	Config     map[string]any  `json:"-"` // flattened
	When       string          `json:"when,omitempty"`
	OnFailure  string          `json:"on_failure,omitempty"`
	Connection *connectionJSON `json:"connection,omitempty"`
}

// MarshalJSON produces {"type": {config + meta}} matching Terraform format.
func (p provisionerJSON) MarshalJSON() ([]byte, error) {
	inner := make(map[string]any, len(p.Config)+3)
	for k, v := range p.Config {
		inner[k] = v
	}
	if p.When != "" {
		inner["when"] = p.When
	}
	if p.OnFailure != "" {
		inner["on_failure"] = p.OnFailure
	}
	if p.Connection != nil {
		inner["connection"] = p.Connection
	}
	return marshalNoEscape(map[string]any{p.Type: inner})
}

// dataSourceConfigJSON represents a single data source block in JSON output.
// Same pattern as resourceConfigJSON but without lifecycle/timeouts/provisioners.
type dataSourceConfigJSON struct {
	Path      string         `json:"-"` // <stackName>/<name>; emitted under "//"
	UniqueID  string         `json:"-"` // <name>; emitted under "//"
	Config    map[string]any `json:"-"` // flattened into top level
	Provider  string         `json:"provider,omitempty"`
	DependsOn []string       `json:"depends_on,omitempty"`
	Count     any            `json:"count,omitempty"`
	ForEach   any            `json:"for_each,omitempty"`
}

// MarshalJSON flattens Config alongside the typed meta-argument fields.
func (d dataSourceConfigJSON) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(d.Config)+5)
	if md := buildResourceMetadata(d.Path, d.UniqueID); md != nil {
		m["//"] = md
	}
	for k, v := range d.Config {
		m[k] = v
	}
	if d.Provider != "" {
		m["provider"] = d.Provider
	}
	if len(d.DependsOn) > 0 {
		m["depends_on"] = d.DependsOn
	}
	if d.Count != nil {
		m["count"] = d.Count
	}
	if d.ForEach != nil {
		m["for_each"] = d.ForEach
	}
	return marshalNoEscape(m)
}

// moduleConfigJSON represents a single module block in JSON output.
type moduleConfigJSON struct {
	Path      string            `json:"-"` // <stackName>/<name>; emitted under "//"
	UniqueID  string            `json:"-"` // <name>; emitted under "//"
	Source    string            `json:"-"` // always set, but serialized in MarshalJSON
	Version   string            `json:"-"`
	Config    map[string]any    `json:"-"` // flattened into top level
	Providers map[string]string `json:"providers,omitempty"`
	DependsOn []string          `json:"depends_on,omitempty"`
	Count     any               `json:"count,omitempty"`
	ForEach   any               `json:"for_each,omitempty"`
}

// MarshalJSON flattens Config alongside the typed meta-argument fields.
func (mc moduleConfigJSON) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(mc.Config)+7)
	if md := buildResourceMetadata(mc.Path, mc.UniqueID); md != nil {
		m["//"] = md
	}
	m["source"] = mc.Source
	if mc.Version != "" {
		m["version"] = mc.Version
	}
	for k, v := range mc.Config {
		m[k] = v
	}
	if len(mc.Providers) > 0 {
		m["providers"] = mc.Providers
	}
	if len(mc.DependsOn) > 0 {
		m["depends_on"] = mc.DependsOn
	}
	if mc.Count != nil {
		m["count"] = mc.Count
	}
	if mc.ForEach != nil {
		m["for_each"] = mc.ForEach
	}
	return marshalNoEscape(m)
}

// providerConfigJSON represents a single provider configuration block.
// Config is flattened alongside the typed alias field.
type providerConfigJSON struct {
	Config map[string]any `json:"-"` // flattened
	Alias  string         `json:"alias,omitempty"`
}

// MarshalJSON flattens Config alongside the alias field.
func (p providerConfigJSON) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(p.Config)+1)
	for k, v := range p.Config {
		m[k] = v
	}
	if p.Alias != "" {
		m["alias"] = p.Alias
	}
	return marshalNoEscape(m)
}

// buildResourceConfig builds a single resource configuration.
// If r.TypedAttributes is set, it is JSON-marshaled and used as the attribute
// payload. Otherwise r.Attributes (the untyped map) is used.
// Setting both is an error.
func buildResourceConfig(r *config.Resource) (resourceConfigJSON, error) {
	attrs, err := resolveAttributes(r.Attributes, r.TypedAttributes, r.Type, r.Name)
	if err != nil {
		return resourceConfigJSON{}, err
	}

	var conn *connectionJSON
	if r.Connection != nil {
		c := buildConnection(r.Connection)
		conn = &c
	}

	provs := make([]provisionerJSON, len(r.Provisioners))
	for i, p := range r.Provisioners {
		provs[i] = buildProvisioner(p)
	}
	if len(provs) == 0 {
		provs = nil
	}

	return resourceConfigJSON{
		Config:       attrs,
		Provider:     r.Provider,
		DependsOn:    r.DependsOn,
		Count:        r.Count,
		ForEach:      r.ForEach,
		Lifecycle:    r.Lifecycle,
		Timeouts:     r.Timeouts,
		Connection:   conn,
		Provisioners: provs,
	}, nil
}

// resolveAttributes returns the attribute map for a resource or data source.
// It enforces that Attributes and TypedAttributes are mutually exclusive
// (a non-nil Attributes map alongside TypedAttributes is an error, even if
// the map is empty — that's still an explicit assignment by the caller),
// and marshals the typed payload to a map when present.
func resolveAttributes(untyped map[string]any, typed any, kindType, kindName string) (map[string]any, error) {
	hasTyped := typed != nil
	hasUntyped := untyped != nil
	if hasTyped && hasUntyped {
		return nil, fmt.Errorf("%s.%s: Attributes and TypedAttributes are mutually exclusive", kindType, kindName)
	}
	if !hasTyped {
		return untyped, nil
	}
	data, err := json.Marshal(typed)
	if err != nil {
		return nil, fmt.Errorf("%s.%s: marshaling TypedAttributes: %w", kindType, kindName, err)
	}
	var attrs map[string]any
	if err := json.Unmarshal(data, &attrs); err != nil {
		return nil, fmt.Errorf("%s.%s: converting TypedAttributes: %w", kindType, kindName, err)
	}
	return attrs, nil
}

// buildDataSources builds the data source configurations. stackName is
// threaded through so each data source's "//.metadata.path" can be set to
// match cdktf's per-resource metadata layout.
func buildDataSources(stackName string, dataSources []config.DataSourcer) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any)

	for _, dd := range dataSources {
		d := dd.AsDataSource()
		if result[d.Type] == nil {
			result[d.Type] = make(map[string]any)
		}

		attrs, err := resolveAttributes(d.Attributes, d.TypedAttributes, d.Type, d.Name)
		if err != nil {
			return nil, err
		}

		result[d.Type][d.SanitizedName()] = dataSourceConfigJSON{
			Path:      stackName + "/" + config.PathifyName(d.Name),
			UniqueID:  d.SanitizedName(),
			Config:    attrs,
			Provider:  d.Provider,
			DependsOn: d.DependsOn,
			Count:     d.Count,
			ForEach:   d.ForEach,
		}
	}

	return result, nil
}

// buildModules builds the module configurations.
func buildModules(stackName string, modules []config.Module) map[string]any {
	result := make(map[string]any, len(modules))

	for _, m := range modules {
		result[m.SanitizedName()] = moduleConfigJSON{
			Path:      stackName + "/" + config.PathifyName(m.Name),
			UniqueID:  m.SanitizedName(),
			Source:    m.Source,
			Version:   m.Version,
			Config:    m.Variables,
			Providers: m.Providers,
			DependsOn: m.DependsOn,
			Count:     m.Count,
			ForEach:   m.ForEach,
		}
	}

	return result
}

// buildVariables builds the variable configurations.
func buildVariables(variables []config.Variable) map[string]variableJSON {
	result := make(map[string]variableJSON)

	for _, v := range variables {
		vj := variableJSON{
			Type:        v.Type,
			Description: v.Description,
			Default:     v.Default,
			Sensitive:   v.Sensitive,
			Nullable:    v.Nullable,
		}

		if v.Validation != nil {
			vj.Validation = v.Validation
		}

		result[v.Name] = vj
	}

	return result
}

// buildOutputs builds the output configurations.
func buildOutputs(outputs []config.Output) map[string]outputJSON {
	result := make(map[string]outputJSON)

	for _, o := range outputs {
		oj := outputJSON{
			Value:       o.Value,
			Description: o.Description,
			Sensitive:   o.Sensitive,
			DependsOn:   o.DependsOn,
		}

		if o.Precondition != nil {
			oj.Precondition = o.Precondition
		}

		result[o.Name] = oj
	}

	return result
}

// buildProvisioner builds a single provisioner block.
func buildProvisioner(p config.Provisioner) provisionerJSON {
	var conn *connectionJSON
	if p.Connection != nil {
		c := buildConnection(p.Connection)
		conn = &c
	}

	return provisionerJSON{
		Type:       p.Type,
		Config:     p.Config,
		When:       string(p.When),
		OnFailure:  string(p.OnFailure),
		Connection: conn,
	}
}

// connectionJSON represents a provisioner connection block.
// Extra fields are flattened into the JSON output alongside the known fields.
type connectionJSON struct {
	Type       string         `json:"type,omitempty"`
	Host       string         `json:"host,omitempty"`
	Port       int            `json:"port,omitempty"`
	User       string         `json:"user,omitempty"`
	Password   string         `json:"password,omitempty"`
	PrivateKey string         `json:"private_key,omitempty"`
	Timeout    string         `json:"timeout,omitempty"`
	Extra      map[string]any `json:"-"` // flattened via MarshalJSON
}

// MarshalJSON flattens Extra fields alongside the known connection fields.
func (c connectionJSON) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)

	if c.Type != "" {
		m["type"] = c.Type
	}
	if c.Host != "" {
		m["host"] = c.Host
	}
	if c.Port != 0 {
		m["port"] = c.Port
	}
	if c.User != "" {
		m["user"] = c.User
	}
	if c.Password != "" {
		m["password"] = c.Password
	}
	if c.PrivateKey != "" {
		m["private_key"] = c.PrivateKey
	}
	if c.Timeout != "" {
		m["timeout"] = c.Timeout
	}

	for k, v := range c.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}

	return marshalNoEscape(m)
}

// buildConnection builds a connection block.
func buildConnection(c *config.Connection) connectionJSON {
	return connectionJSON{
		Type:       c.Type,
		Host:       c.Host,
		Port:       c.Port,
		User:       c.User,
		Password:   c.Password,
		PrivateKey: c.PrivateKey,
		Timeout:    c.Timeout,
		Extra:      c.Extra,
	}
}
