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

	// Providers maps provider configuration aliases for the module.
	// Keys are the provider names expected by the module, values are the provider
	// configurations to use (e.g., {"aws": "aws.west"}).
	Providers map[string]string

	// Variables contains the module input variables.
	Variables map[string]any

	MetaArguments
}

// SanitizedName returns the Terraform-identifier-valid form of m.Name. See
// Resource.SanitizedName for the rationale.
func (m Module) SanitizedName() string {
	return SanitizeName(m.Name)
}

// Address returns the module address (module.sanitized_name).
func (m Module) Address() string {
	return fmt.Sprintf("module.%s", m.SanitizedName())
}

// Ref returns a reference expression to this module's output.
func (m Module) Ref(output string) string {
	return fmt.Sprintf("${module.%s.%s}", m.SanitizedName(), output)
}

// TerraformHclModuleConfig holds the user-facing configuration for a Terraform HCL module call.
// It mirrors the fields available in cdktf.TerraformHclModuleConfig.
type TerraformHclModuleConfig struct {
	// Source is the module source address (e.g., "terraform-aws-modules/eks/aws", "./modules/vpc").
	Source string

	// Version is the module version constraint (e.g., "~> 19.0"). Only for registry modules.
	Version string

	// Variables contains the input variables to pass to the module.
	Variables map[string]any

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

// NewTerraformHclModule creates a Terraform HCL module call and adds it to the stack.
// This provides a cdktf-compatible convenience API over Stack.AddModule.
func NewTerraformHclModule(stack *Stack, name string, cfg TerraformHclModuleConfig) error {
	return stack.AddModule(Module{
		Name:      name,
		Source:    cfg.Source,
		Version:   cfg.Version,
		Providers: cfg.Providers,
		Variables: cfg.Variables,
		MetaArguments: MetaArguments{
			DependsOn: cfg.DependsOn,
			Count:     cfg.Count,
			ForEach:   cfg.ForEach,
		},
	})
}
