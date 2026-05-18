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
	img, err := docker.NewImage(stack, "nginxImage")
	must(err)
	img.Configure(
		docker.Image_SetName("nginx:latest"),
		docker.Image_SetKeepLocally(false),
	)

	container, err := docker.NewContainer(stack, "nginxContainer")
	must(err)
	container.Configure(
		docker.Container_SetImage(nginxImage.Ref("image_id")),
		docker.Container_SetName("tutorial"),
		docker.Container_SetPorts(docker.NewContainerPorts(
			docker.ContainerPorts_SetInternal(80),
			docker.ContainerPorts_SetExternal(8000),
		)),
	)

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
