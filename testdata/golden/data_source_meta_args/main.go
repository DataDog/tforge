// Data source meta-arguments: exercises count, for_each, depends_on,
// and non-default provider references on data sources.
package main

import (
	"fmt"
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func main() {
	stack := config.NewStack("data-source-meta-args-golden")

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

	// Data source with count
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_availability_zones",
		Name:   "counted",
		Attributes: map[string]any{"state": "available"},
		MetaArguments: config.MetaArguments{
			Count: 3,
		},
	}); err != nil {
		log.Fatal(err)
	}

	// Data source with for_each
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_ami",
		Name:   "per_region",
		Attributes: map[string]any{"most_recent": true},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]any{
				"us-east-1": "ami-east",
				"us-west-2": "ami-west",
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	// Data source with depends_on
	if err := stack.AddDataSource(config.DataSource{
		Type:   "aws_caller_identity",
		Name:   "current",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"data.aws_availability_zones.counted"},
		},
	}); err != nil {
		log.Fatal(err)
	}

	// Data source with non-default provider
	if err := stack.AddDataSource(config.DataSource{
		Type:     "aws_vpc",
		Name:     "west_vpc",
		Provider: "aws.west",
		Attributes:   map[string]any{"default": true},
	}); err != nil {
		log.Fatal(err)
	}

	// Data source with all four meta-arguments
	if err := stack.AddDataSource(config.DataSource{
		Type:     "aws_subnet",
		Name:     "all_meta",
		Provider: "aws.west",
		Attributes: map[string]any{
			"vpc_id": "${data.aws_vpc.west_vpc.id}",
		},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"data.aws_vpc.west_vpc"},
			Count:     2,
			ForEach:   nil, // count takes precedence; for_each is nil
		},
	}); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack, "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
