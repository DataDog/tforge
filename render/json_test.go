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

func TestToJSON_Variables(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddVariable(config.Variable{
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
	})

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
	stack.AddOutput(config.Output{
		Name:        "instance_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP",
		Sensitive:   true,
		DependsOn:   []string{"aws_instance.web"},
	})

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
	json.Unmarshal(data, &obj)
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
	json.Unmarshal(data, &obj)
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

	stack.AddVariable(config.Variable{
		Name:    "env",
		Type:    "string",
		Default: "dev",
	})

	stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
	})

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
