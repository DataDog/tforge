// Package config provides a strongly-typed configuration builder for Terraform.
//
// This package allows users to assemble Terraform configurations in Go,
// independent of code generation and provider fetching.
//
// Example usage:
//
//	stack := config.NewStack("my-infrastructure")
//	stack.AddProvider(config.Provider{Name: "aws", Version: "~> 5.0"})
//	stack.AddResource(&config.Resource{Type: "aws_instance", Name: "web"})
package config

import (
	"fmt"
)

// Stack represents a Terraform configuration stack.
// It contains all providers, resources, data sources, and other configuration.
type Stack struct {
	// Name is the stack identifier (used for naming output files).
	Name string

	// Backend configures the Terraform backend for state storage.
	Backend *Backend

	// RequiredProviders specifies provider version constraints.
	RequiredProviders map[string]ProviderRequirement

	// Providers contains provider configurations.
	Providers []Provider

	// Resources contains managed resource configurations.
	// Stored as Resourcer (an interface) so generated typed handles can be
	// added directly and so callers are compiler-enforced to pass pointers.
	// Iterate via .AsResource() to read the underlying *Resource.
	Resources []Resourcer

	// DataSources contains data source configurations. See Resources.
	DataSources []DataSourcer

	// Modules contains module call configurations.
	Modules []Module

	// Variables contains input variable definitions.
	Variables []Variable

	// Outputs contains output value definitions.
	Outputs []Output

	// Locals contains local value definitions.
	Locals map[string]any
}

// NewStack creates a new empty stack with the given name.
// Slices and maps are left nil and initialized lazily by the Add*/Set* methods.
func NewStack(name string) *Stack {
	return &Stack{Name: name}
}

// SetBackend configures the Terraform backend.
func (s *Stack) SetBackend(backend Backend) {
	s.Backend = &backend
}

// AddProvider adds a provider configuration to the stack.
func (s *Stack) AddProvider(p Provider) {
	s.Providers = append(s.Providers, p)

	// Auto-add to required providers if version specified
	if p.Version != "" && p.Source != "" {
		if s.RequiredProviders == nil {
			s.RequiredProviders = make(map[string]ProviderRequirement)
		}
		s.RequiredProviders[p.Name] = ProviderRequirement{
			Source:  p.Source,
			Version: p.Version,
		}
	}
}

// AddResource adds a managed resource to the stack. Accepts any Resourcer:
// commonly *Resource for untyped callers, or a generated typed handle (e.g.
// *Monitor) which carries a back-pointer to the underlying *Resource.
// Mutations to the pointed-to Resource after AddResource returns are reflected
// in the stack and the rendered output.
// Returns an error if r is nil, the underlying *Resource is nil, Type or Name
// is empty, or on duplicate address.
func (s *Stack) AddResource(r Resourcer) error {
	if r == nil {
		return fmt.Errorf("resource is nil")
	}
	res := r.AsResource()
	if res == nil {
		return fmt.Errorf("resource is nil")
	}
	if res.Type == "" {
		return fmt.Errorf("resource type is required")
	}
	if res.Name == "" {
		return fmt.Errorf("resource name is required")
	}

	addr := res.Address()
	for _, existing := range s.Resources {
		if existing.AsResource().Address() == addr {
			return fmt.Errorf("duplicate resource: %s", addr)
		}
	}

	s.Resources = append(s.Resources, r)
	return nil
}

// AddDataSource adds a data source to the stack. See AddResource for the
// rationale of the interface parameter type.
func (s *Stack) AddDataSource(d DataSourcer) error {
	if d == nil {
		return fmt.Errorf("data source is nil")
	}
	ds := d.AsDataSource()
	if ds == nil {
		return fmt.Errorf("data source is nil")
	}
	if ds.Type == "" {
		return fmt.Errorf("data source type is required")
	}
	if ds.Name == "" {
		return fmt.Errorf("data source name is required")
	}

	addr := ds.Address()
	for _, existing := range s.DataSources {
		if existing.AsDataSource().Address() == addr {
			return fmt.Errorf("duplicate data source: %s", addr)
		}
	}

	s.DataSources = append(s.DataSources, d)
	return nil
}

// AddModule adds a module call to the stack.
// It returns an error if the module name or source is empty,
// or if a module with the same name already exists.
func (s *Stack) AddModule(m Module) error {
	if m.Name == "" {
		return fmt.Errorf("module name is required")
	}
	if m.Source == "" {
		return fmt.Errorf("module source is required")
	}

	// Check for duplicates
	for _, existing := range s.Modules {
		if existing.Name == m.Name {
			return fmt.Errorf("duplicate module: %s", m.Name)
		}
	}

	s.Modules = append(s.Modules, m)
	return nil
}

// AddVariable adds an input variable to the stack.
// It returns an error if the variable name is empty or if a variable
// with the same name already exists.
func (s *Stack) AddVariable(v Variable) error {
	if v.Name == "" {
		return fmt.Errorf("variable name is required")
	}
	for _, existing := range s.Variables {
		if existing.Name == v.Name {
			return fmt.Errorf("duplicate variable: %s", v.Name)
		}
	}
	s.Variables = append(s.Variables, v)
	return nil
}

// AddOutput adds an output value to the stack.
// It returns an error if the output name is empty or if an output
// with the same name already exists.
func (s *Stack) AddOutput(o Output) error {
	if o.Name == "" {
		return fmt.Errorf("output name is required")
	}
	for _, existing := range s.Outputs {
		if existing.Name == o.Name {
			return fmt.Errorf("duplicate output: %s", o.Name)
		}
	}
	s.Outputs = append(s.Outputs, o)
	return nil
}

// SetLocal sets a local value.
func (s *Stack) SetLocal(name string, value any) {
	if s.Locals == nil {
		s.Locals = make(map[string]any)
	}
	s.Locals[name] = value
}

// Backend represents a Terraform backend configuration.
type Backend struct {
	// Type is the backend type (e.g., "s3", "gcs", "remote").
	Type string

	// Config contains backend-specific configuration.
	Config map[string]any
}

// ProviderRequirement specifies version constraints for a provider.
type ProviderRequirement struct {
	// Source is the provider source address (e.g., "hashicorp/aws").
	Source string

	// Version is the version constraint (e.g., "~> 5.0").
	Version string
}
