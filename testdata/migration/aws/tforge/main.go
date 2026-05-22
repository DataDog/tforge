// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/hashicorp/terraform-cdk/examples/go/aws/aws"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func NewExampleCdktfGoAwsStack() *config.Stack {
	stack := config.NewStack("ExampleCdktfGoAwsStack")

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "3.70.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	inst, err := aws.NewInstance(stack, "Hello")
	must(err)
	inst.Configure(
		aws.Instance_SetAmi("ami-2757f631"),
		aws.Instance_SetInstanceType("t2.micro"),
		aws.Instance_SetTags(map[string]string{"environment": "development"}),
	)

	must(stack.AddModule(config.Module{
		Name:    "EKS",
		Source:  "terraform-aws-modules/eks/aws",
		Version: "~> 19.0",
		Variables: map[string]any{
			"cluster_name":    "my-eks",
			"cluster_version": "1.17",
			"subnet_ids":      []string{"a", "b"},
			"vpc_id":          "id",
		},
	}))

	return stack
}

func main() {
	stack := NewExampleCdktfGoAwsStack()

	must(render.ToFile(stack, "main.tf.json", ""))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
