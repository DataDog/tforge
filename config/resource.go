package config

import "fmt"

// Resource represents a Terraform managed resource.
type Resource struct {
	// Type is the resource type (e.g., "aws_instance").
	Type string

	// Name is the resource name (unique within the type).
	Name string

	// Provider specifies a non-default provider configuration.
	// Format: "provider.alias" (e.g., "aws.west")
	Provider string

	// Config contains the resource configuration.
	Config map[string]any

	// DependsOn lists explicit dependencies.
	DependsOn []string

	// Count specifies the number of instances (mutually exclusive with ForEach).
	Count any

	// ForEach specifies iteration (mutually exclusive with Count).
	ForEach any

	// Lifecycle contains lifecycle customizations.
	Lifecycle *Lifecycle

	// Connection configures provisioner connections.
	Connection *Connection

	// Provisioners contains provisioner configurations.
	Provisioners []Provisioner

	// Timeouts configures operation timeouts.
	Timeouts *Timeouts
}

// Address returns the resource address (type.name).
func (r Resource) Address() string {
	return fmt.Sprintf("%s.%s", r.Type, r.Name)
}

// Ref returns a reference expression to this resource.
func (r Resource) Ref(attr string) string {
	return fmt.Sprintf("${%s.%s.%s}", r.Type, r.Name, attr)
}

// DataSource represents a Terraform data source.
type DataSource struct {
	// Type is the data source type (e.g., "aws_ami").
	Type string

	// Name is the data source name (unique within the type).
	Name string

	// Provider specifies a non-default provider configuration.
	Provider string

	// Config contains the data source configuration.
	Config map[string]any

	// DependsOn lists explicit dependencies.
	DependsOn []string

	// Count specifies the number of instances.
	Count any

	// ForEach specifies iteration.
	ForEach any
}

// Address returns the data source address (data.type.name).
func (d DataSource) Address() string {
	return fmt.Sprintf("data.%s.%s", d.Type, d.Name)
}

// Ref returns a reference expression to this data source.
func (d DataSource) Ref(attr string) string {
	return fmt.Sprintf("${data.%s.%s.%s}", d.Type, d.Name, attr)
}

// Lifecycle contains resource lifecycle customizations.
type Lifecycle struct {
	// CreateBeforeDestroy creates replacement before destroying original.
	CreateBeforeDestroy bool `json:"create_before_destroy,omitempty"`

	// PreventDestroy prevents resource destruction.
	PreventDestroy bool `json:"prevent_destroy,omitempty"`

	// IgnoreChanges lists attributes to ignore during updates.
	IgnoreChanges []string `json:"ignore_changes,omitempty"`

	// ReplaceTriggeredBy lists references that trigger replacement.
	ReplaceTriggeredBy []string `json:"replace_triggered_by,omitempty"`

	// Precondition defines conditions that must be true.
	Precondition *Condition `json:"precondition,omitempty"`

	// Postcondition defines conditions that must be true after apply.
	Postcondition *Condition `json:"postcondition,omitempty"`
}

// Condition represents a lifecycle condition.
type Condition struct {
	// Condition is the condition expression.
	Condition string `json:"condition"`

	// ErrorMessage is shown when the condition fails.
	ErrorMessage string `json:"error_message"`
}

// Connection configures how provisioners connect to the resource.
type Connection struct {
	// Type is the connection type ("ssh" or "winrm").
	Type string

	// Host is the address to connect to.
	Host string

	// Port is the connection port.
	Port int

	// User is the username for authentication.
	User string

	// Password is the password for authentication.
	Password string

	// PrivateKey is the SSH private key.
	PrivateKey string

	// Timeout is the connection timeout.
	Timeout string

	// Extra contains additional connection attributes.
	Extra map[string]any
}

// ProvisionerWhen specifies when a provisioner should run.
type ProvisionerWhen string

const (
	// WhenCreate runs the provisioner during resource creation (default).
	WhenCreate ProvisionerWhen = "create"

	// WhenDestroy runs the provisioner during resource destruction.
	WhenDestroy ProvisionerWhen = "destroy"
)

// ProvisionerOnFailure specifies behavior when a provisioner fails.
type ProvisionerOnFailure string

const (
	// OnFailureFail aborts on provisioner failure (default).
	OnFailureFail ProvisionerOnFailure = "fail"

	// OnFailureContinue continues on provisioner failure.
	OnFailureContinue ProvisionerOnFailure = "continue"
)

// Provisioner represents a resource provisioner.
type Provisioner struct {
	// Type is the provisioner type (e.g., "local-exec", "remote-exec", "file").
	Type string

	// Config contains provisioner-specific configuration.
	Config map[string]any

	// When specifies when to run (WhenCreate or WhenDestroy).
	When ProvisionerWhen

	// OnFailure specifies behavior on failure (OnFailureFail or OnFailureContinue).
	OnFailure ProvisionerOnFailure

	// Connection overrides the resource connection for this provisioner.
	Connection *Connection
}

// Timeouts configures operation timeouts for a resource.
type Timeouts struct {
	Create string `json:"create,omitempty"`
	Read   string `json:"read,omitempty"`
	Update string `json:"update,omitempty"`
	Delete string `json:"delete,omitempty"`
}
