package main

import (
	"encoding/json"
	"testing"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
)

func TestExample_GeneratesValidJSON(t *testing.T) {
	stack := buildExampleStack()

	data, err := render.ToJSON(stack, "")
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify required Terraform structure
	if _, ok := parsed["terraform"]; !ok {
		t.Error("missing terraform block")
	}
	if _, ok := parsed["provider"]; !ok {
		t.Error("missing provider block")
	}
	if _, ok := parsed["resource"]; !ok {
		t.Error("missing resource block")
	}
}

func buildExampleStack() *config.Stack {
	stack := config.NewStack("datadog-example")

	stack.AddProvider(config.Provider{
		Name:    "datadog",
		Source:  "datadog/datadog",
		Version: "~> 3.39",
		Config: map[string]any{
			"api_url": "https://api.datadoghq.eu/",
		},
	})

	stack.AddResource(config.Resource{
		Type: "datadog_monitor",
		Name: "cpu_alert",
		Config: map[string]any{
			"name":    "High CPU Usage Alert",
			"type":    "metric alert",
			"message": "CPU usage is above threshold. @slack-alerts",
			"query":   "avg(last_5m):avg:system.cpu.user{*} by {host} > 80",
			"tags":    []string{"env:production", "managed-by:tforge"},
			"monitor_thresholds": []map[string]string{
				{"critical": "80", "warning": "60"},
			},
			"notify_no_data":      true,
			"no_data_timeframe":   10,
			"renotify_interval":   60,
			"require_full_window": true,
		},
	})

	return stack
}
