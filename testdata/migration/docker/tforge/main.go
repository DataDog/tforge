// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/hashicorp/terraform-cdk/examples/go/docker/docker"

	"tforge/config"
	"tforge/render"
)

func NewExampleCdktfGoDockerStack() *config.Stack {
	stack := config.NewStack("ExampleCdktfGoDockerStack")

	stack.AddProvider(config.Provider{
		Name:    "docker",
		Source:  "kreuzwerker/docker",
		Version: "3.0.2",
	})

	nginxImage := config.Resource{Type: "docker_image", Name: "nginxImage"}
	must(docker.NewDockerImage(stack, "nginxImage", docker.DockerImage{
		Name:        "nginx:latest",
		KeepLocally: false,
	}))

	must(docker.NewDockerContainer(stack, "nginxContainer", docker.DockerContainer{
		Image: nginxImage.Ref("image_id"),
		Name:  "tutorial",
		Ports: []*docker.DockerContainerPorts{{
			Internal: 80, External: 8000,
		}},
	}))

	return stack
}

func main() {
	stack := NewExampleCdktfGoDockerStack()

	must(render.ToFile(stack, "main.tf.json"))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
