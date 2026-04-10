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
	Condition string `json:"condition"`

	// ErrorMessage is shown when validation fails.
	ErrorMessage string `json:"error_message"`
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
