package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"tforge/config"
)

var update = flag.Bool("update", false, "update golden files")

func TestSnapshot_FullStack(t *testing.T) {
	stack := buildFullStack()

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	golden := filepath.Join("testdata", "full_stack.golden.json")

	if *update {
		if err := os.WriteFile(golden, data, 0644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		t.Log("updated golden file")
		return
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}

	if string(data) != string(expected) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", golden, data, expected)
	}
}

func TestSnapshot_FullStack_ValidJSON(t *testing.T) {
	stack := buildFullStack()

	data, err := ToJSON(stack)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	// Verify round-trip: unmarshal and re-marshal should produce equivalent JSON
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify all expected top-level keys
	for _, key := range []string{"terraform", "provider", "resource", "data", "variable", "output", "locals"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing top-level key: %s", key)
		}
	}
}

func buildFullStack() *config.Stack {
	stack := config.NewStack("snapshot-test")

	stack.SetBackend(config.Backend{
		Type: "s3",
		Config: map[string]any{
			"bucket": "my-state",
			"key":    "terraform.tfstate",
			"region": "us-east-1",
		},
	})

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	if err := stack.AddResource(config.Resource{
		Type: "aws_instance",
		Name: "web",
		Config: map[string]any{
			"ami":           "ami-123456",
			"instance_type": "t3.micro",
			"tags":          map[string]string{"Name": "web-server"},
		},
	}); err != nil {
		panic(err)
	}

	if err := stack.AddDataSource(config.DataSource{
		Type: "aws_ami",
		Name: "ubuntu",
		Config: map[string]any{
			"most_recent": true,
			"owners":      []string{"099720109477"},
		},
	}); err != nil {
		panic(err)
	}

	stack.AddVariable(config.Variable{
		Name:        "environment",
		Type:        "string",
		Description: "Deployment environment",
		Default:     "dev",
	})

	stack.AddOutput(config.Output{
		Name:        "instance_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP address",
	})

	stack.SetLocal("name", "my-app")

	return stack
}
