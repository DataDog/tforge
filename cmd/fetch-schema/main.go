// Command fetch-schema fetches a provider schema and outputs it as JSON.
//
// Usage:
//
//	go run ./cmd/fetch-schema -provider datadog/datadog -version 3.39.0 -output /tmp/schema.json
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"tforge/provider"
)

func main() {
	providerSource := flag.String("provider", "datadog/datadog", "Provider source (namespace/name)")
	version := flag.String("version", "3.39.0", "Provider version")
	output := flag.String("output", "/tmp/schema.json", "Output file path")
	flag.Parse()

	log.Printf("Fetching schema for %s version %s...", *providerSource, *version)

	schema, err := provider.FetchSchema(*providerSource, *version)
	if err != nil {
		log.Fatalf("Failed to fetch schema: %v", err)
	}

	log.Printf("Schema fetched successfully!")
	log.Printf("  - Resources: %d", len(schema.Provider.ResourceSchemas))
	log.Printf("  - Data sources: %d", len(schema.Provider.DataSourceSchemas))

	// Save to file
	if err := provider.SaveSchemaToFile(schema, *output); err != nil {
		log.Fatalf("Failed to save schema: %v", err)
	}

	log.Printf("Schema saved to %s", *output)

	// Print some sample resource names
	fmt.Println("\nSample resources:")
	count := 0
	for name := range schema.Provider.ResourceSchemas {
		fmt.Printf("  - %s\n", name)
		count++
		if count >= 10 {
			fmt.Printf("  ... and %d more\n", len(schema.Provider.ResourceSchemas)-10)
			break
		}
	}

	fmt.Println("\nSample data sources:")
	count = 0
	for name := range schema.Provider.DataSourceSchemas {
		fmt.Printf("  - %s\n", name)
		count++
		if count >= 10 {
			fmt.Printf("  ... and %d more\n", len(schema.Provider.DataSourceSchemas)-10)
			break
		}
	}

	os.Exit(0)
}
