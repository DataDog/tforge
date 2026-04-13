package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tforge/config"
)

func TestToJSON_NilStack(t *testing.T) {
	_, err := ToJSON(nil)
	if err == nil {
		t.Fatal("expected error for nil stack, got nil")
	}
}

func TestToJSON_EmptyStack(t *testing.T) {
	stack := config.NewStack("empty")
	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}
	// Should produce minimal JSON (empty object)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestToJSON_ProviderNoAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Source: "hashicorp/aws",
		Config: map[string]any{"region": "us-east-1"},
	})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	providers, ok := obj["provider"]
	if !ok {
		t.Fatal("missing provider key")
	}
	awsList := providers.(map[string]any)["aws"].([]any)
	if len(awsList) != 1 {
		t.Fatalf("expected 1 aws provider, got %d", len(awsList))
	}
	awsCfg := awsList[0].(map[string]any)
	if awsCfg["region"] != "us-east-1" {
		t.Errorf("expected region us-east-1, got %v", awsCfg["region"])
	}
	if _, exists := awsCfg["alias"]; exists {
		t.Error("alias should not be present when empty")
	}
}

func TestToJSON_ProviderWithAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Alias:  "west",
		Config: map[string]any{"region": "us-west-2"},
	})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	awsList := obj["provider"].(map[string]any)["aws"].([]any)
	awsCfg := awsList[0].(map[string]any)
	if awsCfg["alias"] != "west" {
		t.Errorf("expected alias west, got %v", awsCfg["alias"])
	}
}

func TestToJSON_ProviderWithConfig(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config: map[string]any{
			"region":  "eu-west-1",
			"profile": "dev",
		},
	})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	awsList := obj["provider"].(map[string]any)["aws"].([]any)
	awsCfg := awsList[0].(map[string]any)
	if awsCfg["region"] != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %v", awsCfg["region"])
	}
	if awsCfg["profile"] != "dev" {
		t.Errorf("expected profile dev, got %v", awsCfg["profile"])
	}
}

func TestToJSON_ResourceBasic(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123", "instance_type": "t3.micro"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	if res["ami"] != "ami-123" {
		t.Errorf("expected ami ami-123, got %v", res["ami"])
	}
}

func TestToJSON_ResourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:      "aws_instance",
		Name:      "web",
		Config:    map[string]any{},
		DependsOn: []string{"aws_vpc.main"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	deps := res["depends_on"].([]any)
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_ResourceCount(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Count:  3,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	// JSON numbers unmarshal as float64
	if res["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", res["count"])
	}
}

func TestToJSON_ResourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:    "aws_instance",
		Name:    "web",
		Config:  map[string]any{},
		ForEach: map[string]any{"a": "1", "b": "2"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	forEach := res["for_each"].(map[string]any)
	if forEach["a"] != "1" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestToJSON_ResourceLifecycle(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Lifecycle: &config.Lifecycle{
			CreateBeforeDestroy: true,
			PreventDestroy:      true,
			IgnoreChanges:       []string{"tags"},
			ReplaceTriggeredBy:  []string{"null_resource.trigger"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	lc := res["lifecycle"].(map[string]any)
	if lc["create_before_destroy"] != true {
		t.Error("expected create_before_destroy true")
	}
	if lc["prevent_destroy"] != true {
		t.Error("expected prevent_destroy true")
	}
	ignoreChanges := lc["ignore_changes"].([]any)
	if len(ignoreChanges) != 1 || ignoreChanges[0] != "tags" {
		t.Errorf("unexpected ignore_changes: %v", ignoreChanges)
	}
}

func TestToJSON_ResourceTimeouts(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Timeouts: &config.Timeouts{
			Create: "30m",
			Delete: "10m",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	timeouts := res["timeouts"].(map[string]any)
	if timeouts["create"] != "30m" {
		t.Errorf("expected create 30m, got %v", timeouts["create"])
	}
	if timeouts["delete"] != "10m" {
		t.Errorf("expected delete 10m, got %v", timeouts["delete"])
	}
	if _, ok := timeouts["read"]; ok {
		t.Error("read should not be present when empty")
	}
}

func TestToJSON_ResourceProvisioners(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Provisioners: []config.Provisioner{
			{
				Type:      "local-exec",
				Config:    map[string]any{"command": "echo hello"},
				When:      "create",
				OnFailure: "continue",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	provs := res["provisioner"].([]any)
	if len(provs) != 1 {
		t.Fatalf("expected 1 provisioner, got %d", len(provs))
	}
	localExec := provs[0].(map[string]any)["local-exec"].(map[string]any)
	if localExec["command"] != "echo hello" {
		t.Errorf("expected command 'echo hello', got %v", localExec["command"])
	}
	if localExec["when"] != "create" {
		t.Errorf("expected when 'create', got %v", localExec["when"])
	}
	if localExec["on_failure"] != "continue" {
		t.Errorf("expected on_failure 'continue', got %v", localExec["on_failure"])
	}
}

func TestToJSON_MultipleResourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-web"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "api",
		Config: map[string]any{"ami": "ami-api"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	instances := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)

	web := instances["web"].(map[string]any)
	if web["ami"] != "ami-web" {
		t.Errorf("expected web ami 'ami-web', got %v", web["ami"])
	}

	api := instances["api"].(map[string]any)
	if api["ami"] != "ami-api" {
		t.Errorf("expected api ami 'ami-api', got %v", api["ami"])
	}
}

func TestToJSON_MultipleResourceTypes(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(config.Resource{
		Type:   "aws_vpc",
		Name:   "main",
		Config: map[string]any{"cidr_block": "10.0.0.0/16"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(config.Resource{
		Type:   "aws_subnet",
		Name:   "public",
		Config: map[string]any{"cidr_block": "10.0.1.0/24"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	resources := obj["resource"].(map[string]any)
	if _, ok := resources["aws_instance"]; !ok {
		t.Error("missing aws_instance resource type")
	}
	if _, ok := resources["aws_vpc"]; !ok {
		t.Error("missing aws_vpc resource type")
	}
	if _, ok := resources["aws_subnet"]; !ok {
		t.Error("missing aws_subnet resource type")
	}
}

func TestToJSON_MultipleDataSourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "ubuntu",
		Config: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "amazon_linux",
		Config: map[string]any{"most_recent": false},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	amis := obj["data"].(map[string]any)["aws_ami"].(map[string]any)

	ubuntu := amis["ubuntu"].(map[string]any)
	if ubuntu["most_recent"] != true {
		t.Errorf("expected ubuntu most_recent true, got %v", ubuntu["most_recent"])
	}

	amazonLinux := amis["amazon_linux"].(map[string]any)
	if amazonLinux["most_recent"] != false {
		t.Errorf("expected amazon_linux most_recent false, got %v", amazonLinux["most_recent"])
	}
}

func TestToJSON_DataSource(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "latest",
		Config: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := obj["data"].(map[string]any)["aws_ami"].(map[string]any)["latest"].(map[string]any)
	if ds["most_recent"] != true {
		t.Errorf("expected most_recent true, got %v", ds["most_recent"])
	}
}

func TestToJSON_DataSourceProvider(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:     "aws_ami",
		Name:     "latest",
		Config:   map[string]any{"most_recent": true},
		Provider: "aws.west",
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := obj["data"].(map[string]any)["aws_ami"].(map[string]any)["latest"].(map[string]any)
	if ds["provider"] != "aws.west" {
		t.Errorf("expected provider aws.west, got %v", ds["provider"])
	}
}

func TestToJSON_DataSourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:      "aws_ami",
		Name:      "latest",
		Config:    map[string]any{},
		DependsOn: []string{"aws_vpc.main"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := obj["data"].(map[string]any)["aws_ami"].(map[string]any)["latest"].(map[string]any)
	deps := ds["depends_on"].([]any)
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_DataSourceCount(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "latest",
		Config: map[string]any{},
		Count:  3,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := obj["data"].(map[string]any)["aws_ami"].(map[string]any)["latest"].(map[string]any)
	// JSON numbers unmarshal as float64
	if ds["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", ds["count"])
	}
}

func TestToJSON_DataSourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(config.DataSource{
		Type:    "aws_ami",
		Name:    "latest",
		Config:  map[string]any{},
		ForEach: map[string]any{"a": "1", "b": "2"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := obj["data"].(map[string]any)["aws_ami"].(map[string]any)["latest"].(map[string]any)
	forEach := ds["for_each"].(map[string]any)
	if forEach["a"] != "1" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestToJSON_Variables(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddVariable(config.Variable{
		Name:        "instance_type",
		Type:        "string",
		Description: "The instance type",
		Default:     "t3.micro",
		Sensitive:   true,
		Nullable:    true,
		Validation: &config.VariableValidation{
			Condition:    "length(var.instance_type) > 0",
			ErrorMessage: "Must not be empty",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	v := obj["variable"].(map[string]any)["instance_type"].(map[string]any)
	if v["type"] != "string" {
		t.Errorf("expected type string, got %v", v["type"])
	}
	if v["description"] != "The instance type" {
		t.Errorf("unexpected description: %v", v["description"])
	}
	if v["default"] != "t3.micro" {
		t.Errorf("unexpected default: %v", v["default"])
	}
	if v["sensitive"] != true {
		t.Error("expected sensitive true")
	}
	if v["nullable"] != true {
		t.Error("expected nullable true")
	}
	validation := v["validation"].(map[string]any)
	if validation["condition"] != "length(var.instance_type) > 0" {
		t.Errorf("unexpected condition: %v", validation["condition"])
	}
	if validation["error_message"] != "Must not be empty" {
		t.Errorf("unexpected error_message: %v", validation["error_message"])
	}
}

func TestToJSON_Outputs(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddOutput(config.Output{
		Name:        "instance_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP",
		Sensitive:   true,
		DependsOn:   []string{"aws_instance.web"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	o := obj["output"].(map[string]any)["instance_ip"].(map[string]any)
	if o["value"] != "${aws_instance.web.public_ip}" {
		t.Errorf("unexpected value: %v", o["value"])
	}
	if o["description"] != "The public IP" {
		t.Errorf("unexpected description: %v", o["description"])
	}
	if o["sensitive"] != true {
		t.Error("expected sensitive true")
	}
	deps := o["depends_on"].([]any)
	if len(deps) != 1 || deps[0] != "aws_instance.web" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_Locals(t *testing.T) {
	stack := config.NewStack("test")
	stack.SetLocal("env", "production")
	stack.SetLocal("tags", map[string]string{"team": "platform"})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	locals := obj["locals"].(map[string]any)
	if locals["env"] != "production" {
		t.Errorf("unexpected env: %v", locals["env"])
	}
	tags := locals["tags"].(map[string]any)
	if tags["team"] != "platform" {
		t.Errorf("unexpected tags: %v", tags)
	}
}

func TestToJSON_Backend(t *testing.T) {
	stack := config.NewStack("test")
	stack.SetBackend(config.Backend{
		Type: "s3",
		Config: map[string]any{
			"bucket": "my-state",
			"key":    "terraform.tfstate",
			"region": "us-east-1",
		},
	})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	tf := obj["terraform"].(map[string]any)
	backend := tf["backend"].(map[string]any)
	s3 := backend["s3"].(map[string]any)
	if s3["bucket"] != "my-state" {
		t.Errorf("unexpected bucket: %v", s3["bucket"])
	}
}

func TestToJSON_RequiredProviders(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
	})

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	tf := obj["terraform"].(map[string]any)
	rp := tf["required_providers"].(map[string]any)
	aws := rp["aws"].(map[string]any)
	if aws["source"] != "hashicorp/aws" {
		t.Errorf("unexpected source: %v", aws["source"])
	}
	if aws["version"] != "~> 5.0" {
		t.Errorf("unexpected version: %v", aws["version"])
	}
}

func TestToJSON_FullStack(t *testing.T) {
	stack := config.NewStack("full")

	stack.SetBackend(config.Backend{
		Type:   "s3",
		Config: map[string]any{"bucket": "state"},
	})

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "latest",
		Config: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddVariable(config.Variable{
		Name:    "env",
		Type:    "string",
		Default: "dev",
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
	}); err != nil {
		t.Fatal(err)
	}

	stack.SetLocal("name", "my-app")

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify all top-level keys are present
	for _, key := range []string{"terraform", "provider", "resource", "data", "variable", "output", "locals"} {
		if _, ok := obj[key]; !ok {
			t.Errorf("missing top-level key: %s", key)
		}
	}
}

func TestToFile(t *testing.T) {
	stack := config.NewStack("file-test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Config: map[string]any{"region": "us-east-1"},
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf.json")

	if err := ToFile(stack, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	expected, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	if string(fileData) != string(expected) {
		t.Error("file contents do not match ToJSON output")
	}
}

func TestToFile_CreatesDirectory(t *testing.T) {
	stack := config.NewStack("test")

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "main.tf.json")

	if err := ToFile(stack, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}

func TestToJSON_ResourceConnection(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
		Connection: &config.Connection{
			Type:    "ssh",
			Host:    "1.2.3.4",
			User:    "admin",
			Port:    22,
			Timeout: "5m",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "admin" {
		t.Errorf("expected user admin, got %v", conn["user"])
	}
	if conn["port"] != float64(22) {
		t.Errorf("expected port 22, got %v", conn["port"])
	}
	if conn["timeout"] != "5m" {
		t.Errorf("expected timeout 5m, got %v", conn["timeout"])
	}
}

func TestToJSON_ResourceConnectionWithExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Connection: &config.Connection{
			Type: "winrm",
			Host: "10.0.0.1",
			Extra: map[string]any{
				"https":    true,
				"cacert":   "/path/to/ca.pem",
				"insecure": true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "winrm" {
		t.Errorf("expected type winrm, got %v", conn["type"])
	}
	if conn["https"] != true {
		t.Errorf("expected https true, got %v", conn["https"])
	}
	if conn["cacert"] != "/path/to/ca.pem" {
		t.Errorf("expected cacert, got %v", conn["cacert"])
	}
}

func TestToJSON_LifecyclePrecondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Lifecycle: &config.Lifecycle{
			Precondition: &config.Condition{
				Condition:    "var.x > 0",
				ErrorMessage: "x must be positive",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	lc := res["lifecycle"].(map[string]any)
	pre := lc["precondition"].(map[string]any)
	if pre["condition"] != "var.x > 0" {
		t.Errorf("expected condition 'var.x > 0', got %v", pre["condition"])
	}
	if pre["error_message"] != "x must be positive" {
		t.Errorf("expected error_message 'x must be positive', got %v", pre["error_message"])
	}
}

func TestToJSON_LifecyclePostcondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Lifecycle: &config.Lifecycle{
			Postcondition: &config.Condition{
				Condition:    "self.public_ip != \"\"",
				ErrorMessage: "instance must have a public IP",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	lc := res["lifecycle"].(map[string]any)
	post := lc["postcondition"].(map[string]any)
	if post["condition"] != "self.public_ip != \"\"" {
		t.Errorf("expected condition, got %v", post["condition"])
	}
	if post["error_message"] != "instance must have a public IP" {
		t.Errorf("expected error_message, got %v", post["error_message"])
	}
}

func TestToJSON_OutputPrecondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
		Precondition: &config.Condition{
			Condition:    "aws_instance.web.public_ip != \"\"",
			ErrorMessage: "instance must have a public IP",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	o := obj["output"].(map[string]any)["ip"].(map[string]any)
	pre := o["precondition"].(map[string]any)
	if pre["condition"] != "aws_instance.web.public_ip != \"\"" {
		t.Errorf("expected condition, got %v", pre["condition"])
	}
	if pre["error_message"] != "instance must have a public IP" {
		t.Errorf("expected error_message, got %v", pre["error_message"])
	}
}

func TestToJSON_ProvisionerConnection(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{},
		Provisioners: []config.Provisioner{
			{
				Type:   "remote-exec",
				Config: map[string]any{"inline": []string{"echo hello"}},
				Connection: &config.Connection{
					Type:       "ssh",
					Host:       "10.0.0.1",
					User:       "deploy",
					PrivateKey: "file(\"/path/to/key\")",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	provs := res["provisioner"].([]any)
	remoteExec := provs[0].(map[string]any)["remote-exec"].(map[string]any)
	conn := remoteExec["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1, got %v", conn["host"])
	}
	if conn["user"] != "deploy" {
		t.Errorf("expected user deploy, got %v", conn["user"])
	}
	if conn["private_key"] != "file(\"/path/to/key\")" {
		t.Errorf("expected private_key, got %v", conn["private_key"])
	}
}

func TestToJSON_ModuleBasic(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:    "eks",
		Source:  "terraform-aws-modules/eks/aws",
		Version: "~> 19.0",
		Config: map[string]any{
			"cluster_name":    "my-eks",
			"cluster_version": "1.27",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := obj["module"].(map[string]any)["eks"].(map[string]any)
	if mod["source"] != "terraform-aws-modules/eks/aws" {
		t.Errorf("expected source terraform-aws-modules/eks/aws, got %v", mod["source"])
	}
	if mod["version"] != "~> 19.0" {
		t.Errorf("expected version ~> 19.0, got %v", mod["version"])
	}
	if mod["cluster_name"] != "my-eks" {
		t.Errorf("expected cluster_name my-eks, got %v", mod["cluster_name"])
	}
	if mod["cluster_version"] != "1.27" {
		t.Errorf("expected cluster_version 1.27, got %v", mod["cluster_version"])
	}
}

func TestToJSON_ModuleWithProviders(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:   "vpc",
		Source: "terraform-aws-modules/vpc/aws",
		Config: map[string]any{"cidr": "10.0.0.0/16"},
		Providers: map[string]string{
			"aws": "aws.west",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := obj["module"].(map[string]any)["vpc"].(map[string]any)
	providers := mod["providers"].(map[string]any)
	if providers["aws"] != "aws.west" {
		t.Errorf("expected providers.aws = aws.west, got %v", providers["aws"])
	}
}

func TestToJSON_ModuleWithMetaArguments(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:      "server",
		Source:    "./modules/server",
		Config:    map[string]any{"name": "web"},
		DependsOn: []string{"aws_vpc.main"},
		Count:     3,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := obj["module"].(map[string]any)["server"].(map[string]any)
	if mod["source"] != "./modules/server" {
		t.Errorf("expected source ./modules/server, got %v", mod["source"])
	}
	deps := mod["depends_on"].([]any)
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
	if mod["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", mod["count"])
	}
	// version should not be present for local modules
	if _, ok := mod["version"]; ok {
		t.Error("version should not be present when empty")
	}
}

func TestToJSON_ModuleForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:    "bucket",
		Source:  "terraform-aws-modules/s3-bucket/aws",
		ForEach: map[string]string{"logs": "logs", "data": "data"},
		Config:  map[string]any{"bucket": "${each.key}-bucket"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := obj["module"].(map[string]any)["bucket"].(map[string]any)
	forEach := mod["for_each"].(map[string]any)
	if forEach["logs"] != "logs" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestConnectionJSON_ExtraDoesNotOverwriteKnownFields(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			Extra: map[string]any{
				"type": "winrm",
				"host": "evil.example.com",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected known field type=ssh to take precedence, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected known field host=1.2.3.4 to take precedence, got %v", conn["host"])
	}
}

func TestConnectionJSON_NilExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			User: "admin",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "admin" {
		t.Errorf("expected user admin, got %v", conn["user"])
	}
}

func TestConnectionJSON_EmptyExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
		Connection: &config.Connection{
			Type:  "ssh",
			Host:  "1.2.3.4",
			Extra: map[string]any{},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
}

func TestConnectionJSON_ExtraOnlyUnknownFields(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(config.Resource{
		Type:   "aws_instance",
		Name:   "web",
		Config: map[string]any{"ami": "ami-123"},
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			User: "deploy",
			Extra: map[string]any{
				"bastion_host": "10.0.0.1",
				"bastion_user": "bastion",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := obj["resource"].(map[string]any)["aws_instance"].(map[string]any)["web"].(map[string]any)
	conn := res["connection"].(map[string]any)
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "deploy" {
		t.Errorf("expected user deploy, got %v", conn["user"])
	}
	if conn["bastion_host"] != "10.0.0.1" {
		t.Errorf("expected bastion_host 10.0.0.1, got %v", conn["bastion_host"])
	}
	if conn["bastion_user"] != "bastion" {
		t.Errorf("expected bastion_user bastion, got %v", conn["bastion_user"])
	}
}
