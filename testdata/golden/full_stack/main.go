// Full stack: exercises every top-level Terraform block type.
// Backend, provider, resource, data source, variable (with validation),
// output (with depends_on), and locals.
package main

import (
	"fmt"
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func main() {
	stack := config.NewStack("full-stack-golden")

	stack.SetBackend(config.Backend{
		Type: "s3",
		Config: map[string]any{
			"bucket":         "my-terraform-state",
			"key":            "prod/terraform.tfstate",
			"region":         "us-east-1",
			"encrypt":        true,
			"dynamodb_table": "terraform-locks",
		},
	})

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Attributes: map[string]any{
			"ami":           "ami-0c55b159cbfafe1f0",
			"instance_type": "t3.micro",
			"tags": map[string]string{
				"Name":        "web-server",
				"Environment": "production",
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddDataSource(&config.DataSource{
		Type: "aws_ami",
		Name: "ubuntu",
		Attributes: map[string]any{
			"most_recent": true,
			"owners":      []string{"099720109477"},
			"filter": []map[string]any{
				{
					"name":   "name",
					"values": []string{"ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"},
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddVariable(config.Variable{
		Name:        "environment",
		Type:        "string",
		Description: "Deployment environment",
		Default:     "production",
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddVariable(config.Variable{
		Name:      "enable_monitoring",
		Type:      "bool",
		Default:   true,
		Sensitive: false,
		Nullable:  true,
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddOutput(config.Output{
		Name:        "instance_id",
		Value:       "${aws_instance.web.id}",
		Description: "The ID of the web server instance",
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddOutput(config.Output{
		Name:      "instance_private_ip",
		Value:     "${aws_instance.web.private_ip}",
		Sensitive: true,
		DependsOn: []string{"aws_instance.web"},
	}); err != nil {
		log.Fatal(err)
	}

	stack.SetLocal("project_name", "my-app")
	stack.SetLocal("common_tags", map[string]string{
		"Project":   "my-app",
		"ManagedBy": "tforge",
	})

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
