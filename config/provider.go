package config

// Provider represents a Terraform provider configuration.
type Provider struct {
	// Name is the provider name (e.g., "aws", "google").
	Name string

	// Source is the provider source address (e.g., "hashicorp/aws").
	// Optional if using official HashiCorp providers.
	Source string

	// Version is the version constraint (e.g., "~> 5.0", ">= 4.0, < 6.0").
	Version string

	// Alias is used for multiple configurations of the same provider.
	Alias string

	// Config contains provider-specific configuration.
	Config map[string]any
}

// FullSource returns the complete source address.
// If Source is empty, it defaults to "hashicorp/{Name}".
func (p Provider) FullSource() string {
	if p.Source != "" {
		return p.Source
	}
	return "hashicorp/" + p.Name
}

// Address returns the provider address used for resource association.
// Returns "{name}" or "{name}.{alias}" if an alias is set.
func (p Provider) Address() string {
	if p.Alias != "" {
		return p.Name + "." + p.Alias
	}
	return p.Name
}

// Variable represents a Terraform input variable.
type Variable struct {
	// Name is the variable name.
	Name string

	// Type is the variable type (e.g., "string", "number", "list(string)").
	Type string

	// Description describes the variable's purpose.
	Description string

	// Default is the default value if not provided.
	Default any

	// Sensitive marks the variable as sensitive.
	Sensitive bool

	// Nullable allows the variable to be null.
	Nullable bool

	// Validation contains custom validation rules.
	Validation *VariableValidation
}

// VariableValidation defines custom validation for a variable.
type VariableValidation struct {
	// Condition is the validation expression.
	Condition string

	// ErrorMessage is shown when validation fails.
	ErrorMessage string
}

// Output represents a Terraform output value.
type Output struct {
	// Name is the output name.
	Name string

	// Value is the output value expression.
	Value any

	// Description describes the output's purpose.
	Description string

	// Sensitive marks the output as sensitive.
	Sensitive bool

	// DependsOn lists explicit dependencies.
	DependsOn []string

	// Precondition defines conditions for the output.
	Precondition *Condition
}

// Module represents a Terraform module call.
type Module struct {
	// Name is the local name for this module instance.
	Name string

	// Source is the module source (path, registry, git, etc.).
	Source string

	// Version is the module version (for registry modules).
	Version string

	// Config contains module input variables.
	Config map[string]any

	// Providers maps provider configurations to the module.
	Providers map[string]string

	// DependsOn lists explicit dependencies.
	DependsOn []string

	// Count specifies the number of instances.
	Count any

	// ForEach specifies iteration.
	ForEach any
}

// Address returns the module address.
func (m Module) Address() string {
	return "module." + m.Name
}

// Ref returns a reference to a module output.
func (m Module) Ref(output string) string {
	return "${module." + m.Name + "." + output + "}"
}

// TerraformSettings represents the terraform {} block settings.
type TerraformSettings struct {
	// RequiredVersion specifies the Terraform version constraint.
	RequiredVersion string

	// RequiredProviders maps provider names to requirements.
	RequiredProviders map[string]ProviderRequirement

	// Backend configures state storage.
	Backend *Backend

	// Cloud configures Terraform Cloud/Enterprise.
	Cloud *CloudConfig

	// Experiments enables experimental features.
	Experiments []string
}

// CloudConfig configures Terraform Cloud or Enterprise.
type CloudConfig struct {
	// Organization is the Terraform Cloud organization.
	Organization string

	// Workspaces configures workspace selection.
	Workspaces CloudWorkspaces

	// Hostname is the Terraform Enterprise hostname.
	Hostname string

	// Token is the API token (prefer env var TF_TOKEN_*).
	Token string
}

// CloudWorkspaces configures workspace selection for Terraform Cloud.
type CloudWorkspaces struct {
	// Name is a specific workspace name.
	Name string

	// Tags selects workspaces by tag.
	Tags []string

	// Project selects workspaces by project.
	Project string
}

