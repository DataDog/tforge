// Multi-provider: exercises provider aliases, multiple providers,
// count, for_each, and cross-provider resource references.
package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
)

func main() {
	stack := config.NewStack("multi-provider-golden")

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	stack.AddProvider(config.Provider{
		Name:   "aws",
		Alias:  "west",
		Config: map[string]any{"region": "us-west-2"},
	})

	stack.AddProvider(config.Provider{
		Name:    "random",
		Source:  "hashicorp/random",
		Version: "~> 3.5",
	})

	if err := stack.AddResource(config.Resource{
		Type: "random_pet",
		Name: "name",
		Config: map[string]any{
			"length": 2,
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddResource(config.Resource{
		Type:     "aws_instance",
		Name:     "east",
		Provider: "aws",
		Config: map[string]any{
			"ami":           "ami-east-123",
			"instance_type": "t3.micro",
			"tags": map[string]string{
				"Name": "${random_pet.name.id}-east",
			},
		},
		DependsOn: []string{"random_pet.name"},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddResource(config.Resource{
		Type:     "aws_instance",
		Name:     "west",
		Provider: "aws.west",
		Count:    3,
		Config: map[string]any{
			"ami":           "ami-west-456",
			"instance_type": "t3.micro",
			"tags": map[string]string{
				"Name": "${random_pet.name.id}-west-${count.index}",
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddResource(config.Resource{
		Type:    "aws_s3_bucket",
		Name:    "logs",
		ForEach: `toset(["access-logs", "app-logs", "audit-logs"])`,
		Config: map[string]any{
			"bucket": "${each.key}-${random_pet.name.id}",
			"tags": map[string]string{
				"Purpose": "${each.key}",
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddDataSource(config.DataSource{
		Type:     "aws_availability_zones",
		Name:     "available",
		Provider: "aws.west",
		Config: map[string]any{
			"state": "available",
		},
	}); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
