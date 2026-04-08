package main

import (
	"encoding/json"
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
)

func main() {
	stack := config.NewStack("datadog-v4-golden")

	stack.AddProvider(config.Provider{
		Name:    "datadog",
		Source:  "datadog/datadog",
		Version: "~> 4.4",
		Config: map[string]any{
			"api_url": "https://api.datadoghq.eu/",
		},
	})

	// datadog_compliance_custom_framework is a v4-only resource
	// (not available in datadog provider 3.x).
	framework := DatadogComplianceCustomFramework{
		Handle:  "tforge-security",
		Name:    "tforge Security Framework",
		Version: "1.0.0",
		IconURL: "https://example.com/icon.png",
		Requirements: []*DatadogComplianceCustomFrameworkRequirements{
			{
				Name: "access-control",
				Controls: []*DatadogComplianceCustomFrameworkRequirementsControls{
					{
						Name:    "mfa-required",
						RulesID: []string{"rule-001", "rule-002"},
					},
					{
						Name:    "least-privilege",
						RulesID: []string{"rule-003"},
					},
				},
			},
			{
				Name: "data-protection",
				Controls: []*DatadogComplianceCustomFrameworkRequirementsControls{
					{
						Name:    "encryption-at-rest",
						RulesID: []string{"rule-010"},
					},
				},
			},
		},
	}

	if err := stack.AddResource(config.Resource{
		Type:   "datadog_compliance_custom_framework",
		Name:   "security",
		Config: structToMap(framework),
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
