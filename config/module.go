package config

import "fmt"

// Module represents a Terraform module call.
type Module struct {
	// Name is the module label (unique within the stack).
	Name string

	// Source is the module source address (e.g., "terraform-aws-modules/eks/aws", "./modules/vpc").
	Source string

	// Version is the module version constraint (e.g., "~> 19.0"). Only for registry modules.
	Version string

	// Config contains the module input variables.
	Config map[string]any

	// Providers maps provider configuration aliases for the module.
	// Keys are the provider names expected by the module, values are the provider
	// configurations to use (e.g., {"aws": "aws.west"}).
	Providers map[string]string

	// DependsOn lists explicit dependencies.
	DependsOn []string

	// Count specifies the number of instances (mutually exclusive with ForEach).
	Count any

	// ForEach specifies iteration (mutually exclusive with Count).
	ForEach any
}

// Address returns the module address (module.name).
func (m Module) Address() string {
	return fmt.Sprintf("module.%s", m.Name)
}

// Ref returns a reference expression to this module's output.
func (m Module) Ref(output string) string {
	return fmt.Sprintf("${module.%s.%s}", m.Name, output)
}
