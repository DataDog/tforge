package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
	"tforge/testdata/golden/datadog_v4/datadog"
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
	framework := datadog.ComplianceCustomFramework{
		Handle:  "tforge-security",
		Name:    "tforge Security Framework",
		Version: "1.0.0",
		IconURL: "https://example.com/icon.png",
		Requirements: []*datadog.ComplianceCustomFrameworkRequirements{
			{
				Name: "access-control",
				Controls: []*datadog.ComplianceCustomFrameworkRequirementsControls{
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
				Controls: []*datadog.ComplianceCustomFrameworkRequirementsControls{
					{
						Name:    "encryption-at-rest",
						RulesID: []string{"rule-010"},
					},
				},
			},
		},
	}

	if err := datadog.NewComplianceCustomFramework(stack, "security", datadog.FromComplianceCustomFramework(framework)); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack, "")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
