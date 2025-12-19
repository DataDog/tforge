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
	"sort"

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

// terraformJSON represents the root structure of a .tf.json file.
type terraformJSON struct {
	Terraform *terraformBlock          `json:"terraform,omitempty"`
	Provider  map[string][]any         `json:"provider,omitempty"`
	Resource  map[string]map[string]any `json:"resource,omitempty"`
	Data      map[string]map[string]any `json:"data,omitempty"`
	Variable  map[string]any           `json:"variable,omitempty"`
	Output    map[string]any           `json:"output,omitempty"`
	Locals    map[string]any           `json:"locals,omitempty"`
	Module    map[string]any           `json:"module,omitempty"`
}

// terraformBlock represents the terraform {} block.
type terraformBlock struct {
	RequiredVersion   string                 `json:"required_version,omitempty"`
	RequiredProviders map[string]any         `json:"required_providers,omitempty"`
	Backend           map[string]any         `json:"backend,omitempty"`
	Cloud             map[string]any         `json:"cloud,omitempty"`
}

// buildTerraformJSON converts a Stack to the terraform JSON structure.
func buildTerraformJSON(stack *config.Stack) *terraformJSON {
	tf := &terraformJSON{}

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
		tb.RequiredProviders = make(map[string]any)
		for name, req := range stack.RequiredProviders {
			tb.RequiredProviders[name] = map[string]string{
				"source":  req.Source,
				"version": req.Version,
			}
		}
	}

	// Backend
	if stack.Backend != nil {
		tb.Backend = map[string]any{
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

	// Sort resource types for deterministic output
	sortedResult := make(map[string]map[string]any)
	types := make([]string, 0, len(result))
	for t := range result {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		sortedResult[t] = result[t]
	}

	return sortedResult
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

// buildVariables builds the variable configurations.
func buildVariables(variables []config.Variable) map[string]any {
	result := make(map[string]any)

	for _, v := range variables {
		cfg := make(map[string]any)

		if v.Type != "" {
			cfg["type"] = v.Type
		}

		if v.Description != "" {
			cfg["description"] = v.Description
		}

		if v.Default != nil {
			cfg["default"] = v.Default
		}

		if v.Sensitive {
			cfg["sensitive"] = true
		}

		if v.Nullable {
			cfg["nullable"] = true
		}

		if v.Validation != nil {
			cfg["validation"] = map[string]string{
				"condition":     v.Validation.Condition,
				"error_message": v.Validation.ErrorMessage,
			}
		}

		result[v.Name] = cfg
	}

	return result
}

// buildOutputs builds the output configurations.
func buildOutputs(outputs []config.Output) map[string]any {
	result := make(map[string]any)

	for _, o := range outputs {
		cfg := make(map[string]any)

		cfg["value"] = o.Value

		if o.Description != "" {
			cfg["description"] = o.Description
		}

		if o.Sensitive {
			cfg["sensitive"] = true
		}

		if len(o.DependsOn) > 0 {
			cfg["depends_on"] = o.DependsOn
		}

		result[o.Name] = cfg
	}

	return result
}

// buildLifecycle builds the lifecycle block.
func buildLifecycle(lc *config.Lifecycle) map[string]any {
	cfg := make(map[string]any)

	if lc.CreateBeforeDestroy {
		cfg["create_before_destroy"] = true
	}

	if lc.PreventDestroy {
		cfg["prevent_destroy"] = true
	}

	if len(lc.IgnoreChanges) > 0 {
		cfg["ignore_changes"] = lc.IgnoreChanges
	}

	if len(lc.ReplaceTriggeredBy) > 0 {
		cfg["replace_triggered_by"] = lc.ReplaceTriggeredBy
	}

	return cfg
}

// buildTimeouts builds the timeouts block.
func buildTimeouts(t *config.Timeouts) map[string]any {
	cfg := make(map[string]any)

	if t.Create != "" {
		cfg["create"] = t.Create
	}

	if t.Read != "" {
		cfg["read"] = t.Read
	}

	if t.Update != "" {
		cfg["update"] = t.Update
	}

	if t.Delete != "" {
		cfg["delete"] = t.Delete
	}

	return cfg
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

		prov[p.Type] = cfg
		result[i] = prov
	}

	return result
}

