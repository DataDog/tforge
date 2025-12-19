// Command generate generates Go SDK code from a provider schema JSON file.
//
// Usage:
//
//	go run ./cmd/generate -schema /tmp/schema.json -output /tmp/dist -package datadog
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"tforge/generate"
	"tforge/provider"
)

func main() {
	schemaPath := flag.String("schema", "/tmp/schema.json", "Path to provider schema JSON file")
	outputDir := flag.String("output", "/tmp/dist", "Output directory for generated code")
	packageName := flag.String("package", "datadog", "Go package name for generated code")
	comments := flag.Bool("comments", true, "Generate documentation comments")
	omitEmpty := flag.Bool("omitempty", true, "Add omitempty to optional JSON tags")
	flag.Parse()

	log.Printf("Loading schema from %s...", *schemaPath)

	// Read schema file
	data, err := os.ReadFile(*schemaPath)
	if err != nil {
		log.Fatalf("Failed to read schema file: %v", err)
	}

	// Parse schema
	var ps provider.ProviderSchema
	if err := json.Unmarshal(data, &ps); err != nil {
		log.Fatalf("Failed to parse schema: %v", err)
	}

	schema := &provider.Schema{
		Provider: &ps,
	}

	log.Printf("Schema loaded:")
	log.Printf("  - Resources: %d", len(ps.ResourceSchemas))
	log.Printf("  - Data sources: %d", len(ps.DataSourceSchemas))

	// Create generator
	gen := generate.New(generate.Config{
		OutputDir:        *outputDir,
		PackageName:      *packageName,
		GenerateComments: *comments,
		OmitEmpty:        *omitEmpty,
	})

	log.Printf("Generating Go code to %s...", *outputDir)

	// Generate code
	if err := gen.Generate(schema); err != nil {
		log.Fatalf("Failed to generate code: %v", err)
	}

	log.Printf("Code generation complete!")
	log.Printf("Output files in %s:", *outputDir)

	// List generated files
	entries, err := os.ReadDir(*outputDir)
	if err != nil {
		log.Fatalf("Failed to list output directory: %v", err)
	}

	for _, entry := range entries {
		info, _ := entry.Info()
		if info != nil {
			log.Printf("  - %s (%d bytes)", entry.Name(), info.Size())
		} else {
			log.Printf("  - %s", entry.Name())
		}
	}
}

