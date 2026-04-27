// Lifecycle and provisioners: exercises lifecycle rules, timeouts,
// provisioners with when/on_failure, and complex resource meta-arguments.
package main

import (
	"fmt"
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func main() {
	stack := config.NewStack("lifecycle-provisioners-golden")

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	if err := stack.AddResource(config.Resource{
		Type: "aws_instance",
		Name: "app",
		Lifecycle: &config.Lifecycle{
			CreateBeforeDestroy: true,
			PreventDestroy:      false,
			IgnoreChanges:       []string{"tags", "ami"},
			ReplaceTriggeredBy:  []string{"null_resource.deploy_trigger"},
		},
		Timeouts: &config.Timeouts{
			Create: "30m",
			Update: "20m",
			Delete: "15m",
		},
		Provisioners: []config.Provisioner{
			{
				Type: "local-exec",
				Config: map[string]any{
					"command": "echo 'Instance created: ${self.id}'",
				},
			},
			{
				Type: "remote-exec",
				Config: map[string]any{
					"inline": []string{
						"sudo apt-get update",
						"sudo apt-get install -y nginx",
					},
				},
			},
			{
				Type: "local-exec",
				Config: map[string]any{
					"command": "echo 'Destroying instance: ${self.id}'",
				},
				When:      config.WhenDestroy,
				OnFailure: config.OnFailureContinue,
			},
		},
		Config: map[string]any{
			"ami":           "ami-0c55b159cbfafe1f0",
			"instance_type": "t3.medium",
			"tags": map[string]string{
				"Name": "app-server",
			},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddResource(config.Resource{
		Type: "aws_db_instance",
		Name: "primary",
		Lifecycle: &config.Lifecycle{
			PreventDestroy: true,
			IgnoreChanges:  []string{"password"},
		},
		Timeouts: &config.Timeouts{
			Create: "60m",
			Delete: "45m",
		},
		Config: map[string]any{
			"engine":            "postgres",
			"engine_version":    "15.4",
			"instance_class":    "db.t3.medium",
			"allocated_storage": 50,
			"db_name":           "myapp",
			"username":          "admin",
			"password":          "${var.db_password}",
		},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_instance.app"},
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := stack.AddVariable(config.Variable{
		Name:      "db_password",
		Type:      "string",
		Sensitive: true,
	}); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack, "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
