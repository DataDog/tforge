// Package config provides a strongly-typed configuration builder for Terraform.
//
// This package allows users to assemble Terraform configurations in Go,
// independent of code generation and provider fetching.
//
// Example usage:
//
//	stack := config.NewStack("my-infrastructure")
//	stack.AddProvider(config.Provider{Name: "aws", Version: "~> 5.0"})
//	stack.AddResource(config.Resource{Type: "aws_instance", Name: "web"})
package config

import (
	"fmt"
	"sort"
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
	Resources []Resource

	// DataSources contains data source configurations.
	DataSources []DataSource

	// Variables contains input variable definitions.
	Variables []Variable

	// Outputs contains output value definitions.
	Outputs []Output

	// Locals contains local value definitions.
	Locals map[string]any
}

// NewStack creates a new empty stack with the given name.
func NewStack(name string) *Stack {
	return &Stack{
		Name:              name,
		RequiredProviders: make(map[string]ProviderRequirement),
		Providers:         make([]Provider, 0),
		Resources:         make([]Resource, 0),
		DataSources:       make([]DataSource, 0),
		Variables:         make([]Variable, 0),
		Outputs:           make([]Output, 0),
		Locals:            make(map[string]any),
	}
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
		s.RequiredProviders[p.Name] = ProviderRequirement{
			Source:  p.Source,
			Version: p.Version,
		}
	}
}

// AddResource adds a managed resource to the stack.
func (s *Stack) AddResource(r Resource) error {
	// Validate resource has required fields
	if r.Type == "" {
		return fmt.Errorf("resource type is required")
	}
	if r.Name == "" {
		return fmt.Errorf("resource name is required")
	}

	s.Resources = append(s.Resources, r)
	return nil
}

// AddDataSource adds a data source to the stack.
func (s *Stack) AddDataSource(d DataSource) error {
	if d.Type == "" {
		return fmt.Errorf("data source type is required")
	}
	if d.Name == "" {
		return fmt.Errorf("data source name is required")
	}

	s.DataSources = append(s.DataSources, d)
	return nil
}

// AddVariable adds an input variable to the stack.
func (s *Stack) AddVariable(v Variable) {
	s.Variables = append(s.Variables, v)
}

// AddOutput adds an output value to the stack.
func (s *Stack) AddOutput(o Output) {
	s.Outputs = append(s.Outputs, o)
}

// SetLocal sets a local value.
func (s *Stack) SetLocal(name string, value any) {
	s.Locals[name] = value
}

// ResourceByAddress returns a resource by its address (type.name).
func (s *Stack) ResourceByAddress(address string) *Resource {
	for i := range s.Resources {
		if s.Resources[i].Address() == address {
			return &s.Resources[i]
		}
	}
	return nil
}

// SortedResources returns resources sorted by address.
func (s *Stack) SortedResources() []Resource {
	sorted := make([]Resource, len(s.Resources))
	copy(sorted, s.Resources)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Address() < sorted[j].Address()
	})
	return sorted
}

// SortedDataSources returns data sources sorted by address.
func (s *Stack) SortedDataSources() []DataSource {
	sorted := make([]DataSource, len(s.DataSources))
	copy(sorted, s.DataSources)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Address() < sorted[j].Address()
	})
	return sorted
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

