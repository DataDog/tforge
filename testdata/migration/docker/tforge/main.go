// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/hashicorp/terraform-cdk/examples/go/docker/docker"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func NewExampleCdktfGoDockerStack() *config.Stack {
	stack := config.NewStack("ExampleCdktfGoDockerStack")

	stack.AddProvider(config.Provider{
		Name:    "docker",
		Source:  "kreuzwerker/docker",
		Version: "4.0.0",
	})

	nginxImage := config.Resource{Type: "docker_image", Name: "nginxImage"}
	must(docker.NewImage(stack, "nginxImage",
		docker.Image_SetName("nginx:latest"),
		docker.Image_SetKeepLocally(false),
	))

	must(docker.NewContainer(stack, "nginxContainer",
		docker.Container_SetImage(nginxImage.Ref("image_id")),
		docker.Container_SetName("tutorial"),
		docker.Container_SetPorts(&docker.ContainerPorts{
			Internal: 80, External: 8000,
		}),
	))

	return stack
}

func main() {
	stack := NewExampleCdktfGoDockerStack()

	must(render.ToFile(stack, "main.tf.json", ""))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
