// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func NewMyStack() *config.Stack {
	stack := config.NewStack("cdktf-azure-go")

	stack.AddProvider(config.Provider{
		Name:    "azurerm",
		Source:  "hashicorp/azurerm",
		Version: "2.99.0",
		Config:  map[string]any{"features": map[string]any{}},
	})

	must(stack.AddModule(config.Module{
		Name:    "null",
		Source:  "vancluever/module/null",
		Version: "2.0.2",
		Config: map[string]any{
			"trigger": "one",
		},
	}))

	rg := config.Resource{Type: "azurerm_resource_group", Name: "test_rg"}
	must(stack.AddResource(config.Resource{
		Type: "azurerm_resource_group",
		Name: "test_rg",
		Config: map[string]any{
			"name":     "test-rg",
			"location": "westeurope",
		},
	}))

	vmNw := config.Resource{Type: "azurerm_virtual_network", Name: "test_vm_nw"}
	must(stack.AddResource(config.Resource{
		Type: "azurerm_virtual_network",
		Name: "test_vm_nw",
		Config: map[string]any{
			"name":                "test-vm-nw",
			"address_space":       []string{"10.0.0.0/16"},
			"location":            rg.Ref("location"),
			"resource_group_name": rg.Ref("name"),
		},
	}))

	vmNwSn := config.Resource{Type: "azurerm_subnet", Name: "test_vm_nw_sn"}
	must(stack.AddResource(config.Resource{
		Type: "azurerm_subnet",
		Name: "test_vm_nw_sn",
		Config: map[string]any{
			"name":                 "test-vm-nw-sn",
			"resource_group_name":  rg.Ref("name"),
			"virtual_network_name": vmNw.Ref("name"),
			"address_prefixes":     []string{"10.0.2.0/24"},
		},
	}))

	vmNic := config.Resource{Type: "azurerm_network_interface", Name: "test_vm_nic"}
	must(stack.AddResource(config.Resource{
		Type: "azurerm_network_interface",
		Name: "test_vm_nic",
		Config: map[string]any{
			"name":                "test-vm-nic",
			"location":            rg.Ref("location"),
			"resource_group_name": rg.Ref("name"),
			"ip_configuration": []map[string]any{{
				"name":                          "internal",
				"subnet_id":                     vmNwSn.Ref("id"),
				"private_ip_address_allocation": "Dynamic",
			}},
		},
	}))

	nm := config.Module{Name: "null", Source: "vancluever/module/null"}
	vm := config.Resource{Type: "azurerm_linux_virtual_machine", Name: "test_vm"}
	must(stack.AddResource(config.Resource{
		Type: "azurerm_linux_virtual_machine",
		Name: "test_vm",
		Config: map[string]any{
			"name":                  "test-vm",
			"location":              rg.Ref("location"),
			"resource_group_name":   rg.Ref("name"),
			"size":                  "Standard_F2",
			"admin_username":        "adminuser",
			"network_interface_ids": []string{vmNic.Ref("id")},
			"admin_ssh_key": []map[string]any{{
				"username":   "glados",
				"public_key": `${file("~/.ssh/id_rsa.pub")}`,
			}},
			"os_disk": map[string]any{
				"caching":              "ReadWrite",
				"storage_account_type": "Standard_LRS",
			},
			"source_image_reference": map[string]any{
				"publisher": "Canonical",
				"offer":     "UbuntuServer",
				"sku":       "16.04-LTS",
				"version":   "latest",
			},
		},
	}))

	if err := stack.AddOutput(config.Output{
		Name:  "names",
		Value: []string{vm.Ref("name"), rg.Ref("name"), nm.Ref("null_resource_id")},
	}); err != nil {
		log.Fatal(err)
	}

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
