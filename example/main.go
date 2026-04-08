// Example demonstrates how to use tforge with go:generate.
//
// First, generate the provider SDK:
//
//	go generate ./...
//
// Then run this example:
//
//	go run main.go
package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/example/datadog"
	"tforge/render"
)

func main() {
	// Create a new stack
	stack := config.NewStack("datadog-example")

	// Configure the Datadog provider
	stack.AddProvider(config.Provider{
		Name:    "datadog",
		Source:  "datadog/datadog",
		Version: "~> 3.39",
		Config: map[string]any{
			"api_url": "https://api.datadoghq.eu/",
		},
	})

	// Create a Datadog monitor using generated types and constructor
	monitor := datadog.DatadogMonitor{
		Name:    "High CPU Usage Alert",
		Type:    "metric alert",
		Message: "CPU usage is above threshold. @slack-alerts",
		Query:   "avg(last_5m):avg:system.cpu.user{*} by {host} > 80",
		Tags:    []string{"env:production", "managed-by:tforge"},
		MonitorThresholds: []*datadog.DatadogMonitorMonitorThresholds{
			{
				Critical: "80",
				Warning:  "60",
			},
		},
		NotifyNoData:      true,
		NoDataTimeframe:   10,
		RenotifyInterval:  60,
		RequireFullWindow: true,
	}

	if err := datadog.NewDatadogMonitor(stack, "cpu_alert", monitor); err != nil {
		log.Fatal(err)
	}

	// Render to JSON
	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
