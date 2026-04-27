package main

import (
	"fmt"
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
	"github.com/DataDog/tforge/testdata/golden/datadog/datadog"
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

	if err := datadog.NewMonitor(stack, "cpu_alert", nil,
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
		datadog.Monitor_SetLocked(true), // removed in v4 — validates that ~> 3.39 constraint matters
	); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack, "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
