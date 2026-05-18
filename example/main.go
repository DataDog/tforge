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

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/example/datadog"
	"github.com/DataDog/tforge/render"
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

	// Create a Datadog monitor: the constructor attaches it to the stack
	// and returns a handle. Apply configures the attributes; the handle
	// can be reused later to add more attributes or meta-arguments.
	m, err := datadog.NewMonitor(stack, "cpu_alert")
	if err != nil {
		log.Fatal(err)
	}
	m.Apply(
		datadog.Monitor_SetName("High CPU Usage Alert"),
		datadog.Monitor_SetType("metric alert"),
		datadog.Monitor_SetMessage("CPU usage is above threshold. @slack-alerts"),
		datadog.Monitor_SetQuery("avg(last_5m):avg:system.cpu.user{*} by {host} > 80"),
		datadog.Monitor_SetTags([]string{"env:production", "managed-by:tforge"}),
		datadog.Monitor_SetMonitorThresholds(&datadog.MonitorMonitorThresholds{
			Critical: "80",
			Warning:  "60",
		}),
		datadog.Monitor_SetNotifyNoData(true),
		datadog.Monitor_SetNoDataTimeframe(10),
		datadog.Monitor_SetRenotifyInterval(60),
		datadog.Monitor_SetRequireFullWindow(true),
	)

	// Render to JSON
	data, err := render.ToJSON(stack, "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
