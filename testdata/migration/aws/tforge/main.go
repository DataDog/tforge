// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/hashicorp/terraform-cdk/examples/go/aws/aws"

	"tforge/config"
	"tforge/render"
)

func NewExampleCdktfGoAwsStack() *config.Stack {
	stack := config.NewStack("ExampleCdktfGoAwsStack")

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 3.70",
		Config:  map[string]any{"region": "us-east-1"},
	})

	must(aws.NewAwsInstance(stack, "Hello", aws.AwsInstance{
		Ami:          "ami-2757f631",
		InstanceType: "t2.micro",
		Tags:         map[string]string{"environment": "development"},
	}))

	// Note: original CDKTF example used terraform-aws-modules/eks/aws module.
	// tforge does not support modules — using aws_eks_cluster resource instead.
	must(aws.NewAwsEKSCluster(stack, "EKS", aws.AwsEKSCluster{
		Name:    "my-eks",
		Version: "1.17",
		RoleARN: "arn:aws:iam::012345678910:role/eks-role",
		VPCConfig: []*aws.AwsEKSClusterVPCConfig{
			{SubnetIDS: []string{"subnet-a", "subnet-b"}},
		},
	}))

	return stack
}

func main() {
	stack := NewExampleCdktfGoAwsStack()

	must(render.ToFile(stack, "main.tf.json"))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
