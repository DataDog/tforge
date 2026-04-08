package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
)

func main() {
	stack := config.NewStack("datadog-golden")

	stack.AddProvider(config.Provider{
		Name:    "datadog",
		Source:  "datadog/datadog",
		Version: "~> 3.39",
		Config: map[string]any{
			"api_url": "https://api.datadoghq.eu/",
		},
	})

	monitor := DatadogMonitor{
		Name:    "High CPU Usage Alert",
		Type:    "metric alert",
		Message: "CPU usage is above threshold. @slack-alerts",
		Query:   "avg(last_5m):avg:system.cpu.user{*} by {host} > 80",
		Tags:    []string{"env:production", "managed-by:tforge"},
		MonitorThresholds: []*DatadogMonitorMonitorThresholds{
			{
				Critical: "80",
				Warning:  "60",
			},
		},
		NotifyNoData:      true,
		NoDataTimeframe:   10,
		RenotifyInterval:  60,
		RequireFullWindow: true,
		Locked:            true, // removed in v4 — validates that ~> 3.39 constraint matters
	}

	if err := NewDatadogMonitor(stack, "cpu_alert", monitor); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
