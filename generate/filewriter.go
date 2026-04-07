package generate

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"
)

// WriteFile writes a jennifer file to disk, creating directories as needed.
func WriteFile(f *jen.File, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Render to buffer
	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return fmt.Errorf("rendering file: %w", err)
	}

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted if formatting fails (for debugging)
		formatted = buf.Bytes()
	}

	// Write to file
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}

	return nil
}

// toGoName converts a snake_case Terraform name to PascalCase Go name.
// If the result would start with a digit (invalid Go identifier), it is
// prefixed with "X". An empty input returns an empty string.
func toGoName(tfName string) string {
	parts := strings.Split(tfName, "_")
	var result strings.Builder

	for _, part := range parts {
		if part == "" {
			continue
		}
		// Handle common abbreviations
		upper := strings.ToUpper(part)
		if isCommonAbbreviation(upper) {
			result.WriteString(upper)
		} else {
			result.WriteString(capitalizeFirst(part))
		}
	}

	s := result.String()
	// Prefix with "X" if the result starts with a digit (invalid Go identifier).
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		return "X" + s
	}
	return s
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// commonAbbreviations is the set of abbreviations that should remain
// fully uppercased in Go identifiers (e.g. "ID", "API", "URL").
var commonAbbreviations = map[string]bool{
	"ID":    true,
	"IDS":   true,
	"API":   true,
	"URL":   true,
	"URI":   true,
	"HTTP":  true,
	"HTTPS": true,
	"JSON":  true,
	"XML":   true,
	"HTML":  true,
	"CSS":   true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"UDP":   true,
	"IP":    true,
	"DNS":   true,
	"VPC":   true,
	"ARN":   true,
	"IAM":   true,
	"EC2":   true,
	"ECS":   true,
	"EKS":   true,
	"RDS":   true,
	"S3":    true,
	"SQS":   true,
	"SNS":   true,
	"KMS":   true,
	"ACL":   true,
	"TTL":   true,
	"CPU":   true,
	"RAM":   true,
	"OS":    true,
	"VM":    true,
}

// isCommonAbbreviation returns true if the string is a common abbreviation.
func isCommonAbbreviation(s string) bool {
	return commonAbbreviations[s]
}

// cleanDescription cleans a description string for use in Go comments.
// It normalizes whitespace, removes markdown artifacts, and ensures clean formatting.
func cleanDescription(text string) string {
	// Normalize whitespace - collapse multiple spaces/newlines into single space
	text = strings.Join(strings.Fields(text), " ")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}
