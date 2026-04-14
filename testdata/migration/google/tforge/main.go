// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"tforge/config"
	"tforge/render"
)

func NewMyStack() *config.Stack {
	stack := config.NewStack("google")

	stack.AddProvider(config.Provider{
		Name:    "google",
		Source:  "hashicorp/google",
		Version: "3.69.0",
		Config: map[string]any{
			"zone":    "us-west1",
			"project": "dschmidt-cdk-test",
		},
	})

	stack.AddProvider(config.Provider{
		Name:    "local",
		Source:  "hashicorp/local",
		Version: "2.1.0",
	})

	sa := config.Resource{Type: "google_service_account", Name: "sa"}
	must(stack.AddResource(config.Resource{
		Type: "google_service_account",
		Name: "sa",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"account_id":   "cluster-admin",
				"display_name": "Cluster Admin",
			},
		},
	}))

	cluster := config.Resource{Type: "google_container_cluster", Name: "cluster"}
	must(stack.AddResource(config.Resource{
		Type: "google_container_cluster",
		Name: "cluster",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"name":                     "cluster",
				"remove_default_node_pool": true,
				"initial_node_count":       1,
			},
		},
	}))

	must(stack.AddResource(config.Resource{
		Type: "google_container_node_pool",
		Name: "main-pool",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"name":    "main",
				"cluster": cluster.Ref("name"),
				"node_config": map[string]any{
					"machine_type":    "e2-medium",
					"preemptible":     true,
					"service_account": sa.Ref("email"),
					"oauth_scopes":    []string{"https://www.googleapis.com/auth/cloud-platform"},
				},
			},
		},
	}))

	auth := config.Module{Name: "auth", Source: "terraform-google-modules/kubernetes-engine/google//modules/auth"}
	must(stack.AddModule(config.Module{
		Name:    "auth",
		Source:  "terraform-google-modules/kubernetes-engine/google//modules/auth",
		Version: "14.3.0",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"cluster_name": cluster.Ref("name"),
				"location":     cluster.Ref("location"),
				"project_id":   cluster.Ref("project"),
			},
		},
	}))

	must(stack.AddResource(config.Resource{
		Type: "local_file",
		Name: "kubeconfig",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"filename": "kubeconfig.yaml",
				"content":  auth.Ref("kubeconfig_raw"),
			},
		},
	}))

	stack.AddProvider(config.Provider{
		Name:    "kubernetes",
		Source:  "hashicorp/kubernetes",
		Version: "2.2.0",
		Config: map[string]any{
			"cluster_ca_certificate": auth.Ref("cluster_ca_certificate"),
			"host":                   auth.Ref("host"),
			"token":                  auth.Ref("token"),
		},
	})

	must(stack.AddResource(config.Resource{
		Type: "kubernetes_namespace",
		Name: "namespace",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"metadata": map[string]any{
					"name": "development",
				},
			},
		},
	}))

	stack.AddProvider(config.Provider{
		Name:    "helm",
		Source:  "hashicorp/helm",
		Version: "2.1.2",
		Config: map[string]any{
			"kubernetes": map[string]any{
				"cluster_ca_certificate": auth.Ref("cluster_ca_certificate"),
				"host":                   auth.Ref("host"),
				"token":                  auth.Ref("token"),
			},
		},
	})

	must(stack.AddResource(config.Resource{
		Type: "helm_release",
		Name: "cert-manager",
		MetaArguments: config.MetaArguments{
			Config: map[string]any{
				"name":             "cert-manager",
				"repository":       "https://charts.jetstack.io",
				"chart":            "cert-manager",
				"create_namespace": true,
				"namespace":        "cert-manager",
				"version":          "v1.3.1",
			},
		},
	}))

	return stack
}

func main() {
	stack := NewMyStack()
	must(render.ToFile(stack, "main.tf.json", ""))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
