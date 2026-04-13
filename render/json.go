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
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"tforge/config"
)

// ToJSON renders a stack to JSON bytes.
func ToJSON(stack *config.Stack) ([]byte, error) {
	if stack == nil {
		return nil, fmt.Errorf("nil stack")
	}

	tf := buildTerraformJSON(stack)

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
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// Version is the tforge version, set at build time or defaulting to "dev".
var Version = "dev"

// terraformJSON represents the root structure of a .tf.json file.
type terraformJSON struct {
	Comment   map[string]any            `json:"//,omitempty"`
	Terraform *terraformBlock           `json:"terraform,omitempty"`
	Provider  map[string][]any          `json:"provider,omitempty"`
	Resource  map[string]map[string]any `json:"resource,omitempty"`
	Data      map[string]map[string]any `json:"data,omitempty"`
	Module    map[string]map[string]any `json:"module,omitempty"`
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
func buildTerraformJSON(stack *config.Stack) *terraformJSON {
	tf := &terraformJSON{
		Comment: map[string]any{
			"metadata": map[string]any{
				"tool":      "tforge",
				"version":   Version,
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
		tf.Resource = buildResources(stack.Resources)
	}

	// Build data sources
	if len(stack.DataSources) > 0 {
		tf.Data = buildDataSources(stack.DataSources)
	}

	// Build modules
	if len(stack.Modules) > 0 {
		tf.Module = buildModules(stack.Modules)
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

	return tf
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
		cfg := make(map[string]any)

		// Copy provider config
		for k, v := range p.Config {
			cfg[k] = v
		}

		// Add alias if set
		if p.Alias != "" {
			cfg["alias"] = p.Alias
		}

		result[p.Name] = append(result[p.Name], cfg)
	}

	return result
}

// buildResources builds the resource configurations.
func buildResources(resources []config.Resource) map[string]map[string]any {
	result := make(map[string]map[string]any)

	for _, r := range resources {
		if result[r.Type] == nil {
			result[r.Type] = make(map[string]any)
		}

		cfg := buildResourceConfig(r)
		result[r.Type][r.Name] = cfg
	}

	return result
}

// buildResourceConfig builds a single resource configuration.
func buildResourceConfig(r config.Resource) map[string]any {
	cfg := make(map[string]any)

	// Copy resource config
	for k, v := range r.Config {
		cfg[k] = v
	}

	// Add meta-arguments
	if r.Provider != "" {
		cfg["provider"] = r.Provider
	}

	if len(r.DependsOn) > 0 {
		cfg["depends_on"] = r.DependsOn
	}

	if r.Count != nil {
		cfg["count"] = r.Count
	}

	if r.ForEach != nil {
		cfg["for_each"] = r.ForEach
	}

	if r.Lifecycle != nil {
		cfg["lifecycle"] = buildLifecycle(r.Lifecycle)
	}

	if r.Timeouts != nil {
		cfg["timeouts"] = buildTimeouts(r.Timeouts)
	}

	if r.Connection != nil {
		cfg["connection"] = buildConnection(r.Connection)
	}

	if len(r.Provisioners) > 0 {
		cfg["provisioner"] = buildProvisioners(r.Provisioners)
	}

	return cfg
}

// buildDataSources builds the data source configurations.
func buildDataSources(dataSources []config.DataSource) map[string]map[string]any {
	result := make(map[string]map[string]any)

	for _, d := range dataSources {
		if result[d.Type] == nil {
			result[d.Type] = make(map[string]any)
		}

		cfg := make(map[string]any)

		// Copy data source config
		for k, v := range d.Config {
			cfg[k] = v
		}

		// Add meta-arguments
		if d.Provider != "" {
			cfg["provider"] = d.Provider
		}

		if len(d.DependsOn) > 0 {
			cfg["depends_on"] = d.DependsOn
		}

		if d.Count != nil {
			cfg["count"] = d.Count
		}

		if d.ForEach != nil {
			cfg["for_each"] = d.ForEach
		}

		result[d.Type][d.Name] = cfg
	}

	return result
}

// buildModules builds the module configurations.
func buildModules(modules []config.Module) map[string]map[string]any {
	result := make(map[string]map[string]any)

	for _, m := range modules {
		cfg := make(map[string]any)

		cfg["source"] = m.Source

		if m.Version != "" {
			cfg["version"] = m.Version
		}

		// Copy module input variables
		for k, v := range m.Config {
			cfg[k] = v
		}

		// Add meta-arguments
		if len(m.Providers) > 0 {
			cfg["providers"] = m.Providers
		}

		if len(m.DependsOn) > 0 {
			cfg["depends_on"] = m.DependsOn
		}

		if m.Count != nil {
			cfg["count"] = m.Count
		}

		if m.ForEach != nil {
			cfg["for_each"] = m.ForEach
		}

		result[m.Name] = cfg
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

// buildLifecycle builds the lifecycle block.
func buildLifecycle(lc *config.Lifecycle) *config.Lifecycle {
	return lc
}

// buildTimeouts builds the timeouts block.
func buildTimeouts(t *config.Timeouts) *config.Timeouts {
	return t
}

// buildProvisioners builds the provisioner blocks.
func buildProvisioners(provisioners []config.Provisioner) []map[string]any {
	result := make([]map[string]any, len(provisioners))

	for i, p := range provisioners {
		prov := make(map[string]any)

		cfg := make(map[string]any)
		for k, v := range p.Config {
			cfg[k] = v
		}

		if p.When != "" {
			cfg["when"] = p.When
		}

		if p.OnFailure != "" {
			cfg["on_failure"] = p.OnFailure
		}

		if p.Connection != nil {
			cfg["connection"] = buildConnection(p.Connection)
		}

		prov[p.Type] = cfg
		result[i] = prov
	}

	return result
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

	return json.Marshal(m)
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
