package main

import (
	"encoding/json"
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
	}

	if err := stack.AddResource(config.Resource{
		Type:   "datadog_monitor",
		Name:   "cpu_alert",
		Config: structToMap(monitor),
	}); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}

func structToMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("structToMap: marshal failed: %v", err))
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		panic(fmt.Sprintf("structToMap: unmarshal failed: %v", err))
	}
	return result
}
