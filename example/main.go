// Example demonstrates how to use tforge to build Terraform configurations.
//
// This example creates a simple AWS infrastructure with an EC2 instance
// and renders it to a .tf.json file.
package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
)

func main() {
	// Create a new stack
	stack := config.NewStack("example-infrastructure")

	// Configure the AWS provider
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config: map[string]any{
			"region": "us-east-1",
			"default_tags": map[string]any{
				"tags": map[string]any{
					"Environment": "dev",
					"ManagedBy":   "tforge",
				},
			},
		},
	})

	// Add a second provider with an alias for another region
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Alias:  "west",
		Config: map[string]any{
			"region": "us-west-2",
		},
	})

	// Define input variables
	stack.AddVariable(config.Variable{
		Name:        "instance_type",
		Type:        "string",
		Description: "EC2 instance type",
		Default:     "t3.micro",
	})

	stack.AddVariable(config.Variable{
		Name:        "environment",
		Type:        "string",
		Description: "Environment name",
		Validation: &config.VariableValidation{
			Condition:    `contains(["dev", "staging", "prod"], var.environment)`,
			ErrorMessage: "Environment must be dev, staging, or prod.",
		},
	})

	// Add a data source to get the latest Amazon Linux 2 AMI
	stack.AddDataSource(config.DataSource{
		Type: "aws_ami",
		Name: "amazon_linux",
		Config: map[string]any{
			"most_recent": true,
			"owners":      []string{"amazon"},
			"filter": []map[string]any{
				{
					"name":   "name",
					"values": []string{"amzn2-ami-hvm-*-x86_64-gp2"},
				},
				{
					"name":   "virtualization-type",
					"values": []string{"hvm"},
				},
			},
		},
	})

	// Add a VPC
	stack.AddResource(config.Resource{
		Type: "aws_vpc",
		Name: "main",
		Config: map[string]any{
			"cidr_block":           "10.0.0.0/16",
			"enable_dns_hostnames": true,
			"enable_dns_support":   true,
			"tags": map[string]any{
				"Name": "main-vpc",
			},
		},
	})

	// Add a subnet
	stack.AddResource(config.Resource{
		Type: "aws_subnet",
		Name: "public",
		Config: map[string]any{
			"vpc_id":                  "${aws_vpc.main.id}",
			"cidr_block":              "10.0.1.0/24",
			"availability_zone":       "us-east-1a",
			"map_public_ip_on_launch": true,
			"tags": map[string]any{
				"Name": "public-subnet",
			},
		},
	})

	// Add an EC2 instance
	stack.AddResource(config.Resource{
		Type: "aws_instance",
		Name: "web",
		Config: map[string]any{
			"ami":           "${data.aws_ami.amazon_linux.id}",
			"instance_type": "${var.instance_type}",
			"subnet_id":     "${aws_subnet.public.id}",
			"tags": map[string]any{
				"Name": "web-server",
			},
		},
		Lifecycle: &config.Lifecycle{
			CreateBeforeDestroy: true,
			IgnoreChanges:       []string{"tags"},
		},
		Timeouts: &config.Timeouts{
			Create: "10m",
			Delete: "5m",
		},
	})

	// Add local values
	stack.SetLocal("common_tags", map[string]any{
		"Project": "tforge-example",
		"Owner":   "platform-team",
	})

	// Add outputs
	stack.AddOutput(config.Output{
		Name:        "instance_id",
		Value:       "${aws_instance.web.id}",
		Description: "The ID of the web server instance",
	})

	stack.AddOutput(config.Output{
		Name:        "instance_public_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP of the web server",
		Sensitive:   false,
	})

	stack.AddOutput(config.Output{
		Name:        "vpc_id",
		Value:       "${aws_vpc.main.id}",
		Description: "The ID of the VPC",
	})

	// Render to JSON
	jsonData, err := render.ToJSON(stack)
	if err != nil {
		log.Fatalf("Failed to render stack: %v", err)
	}

	// Print the generated JSON
	fmt.Println("Generated Terraform JSON:")
	fmt.Println(string(jsonData))

	// Write to file
	if err := render.ToFile(stack, "main.tf.json"); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Println("\nWritten to main.tf.json")
}

