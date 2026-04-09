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

	must(aws.NewInstance(stack, "Hello",
		aws.Instance_SetAmi("ami-2757f631"),
		aws.Instance_SetInstanceType("t2.micro"),
		aws.Instance_SetTags(map[string]string{"environment": "development"}),
	))

	// Note: original CDKTF example used terraform-aws-modules/eks/aws module.
	// tforge does not support modules — using aws_eks_cluster resource instead.
	must(aws.NewEKSCluster(stack, "EKS",
		aws.EKSCluster_SetName("my-eks"),
		aws.EKSCluster_SetVersion("1.17"),
		aws.EKSCluster_SetRoleARN("arn:aws:iam::012345678910:role/eks-role"),
		aws.EKSCluster_SetVPCConfig(&aws.EKSClusterVPCConfig{
			SubnetIDS: []string{"subnet-a", "subnet-b"},
		}),
	))

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
