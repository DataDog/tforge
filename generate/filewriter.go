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

	return result.String()
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

// isCommonAbbreviation returns true if the string is a common abbreviation.
func isCommonAbbreviation(s string) bool {
	abbreviations := map[string]bool{
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
	return abbreviations[s]
}

// wrapComment wraps a comment to the specified line width.
func wrapComment(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		if i == 0 {
			result.WriteString(word)
			lineLen = len(word)
			continue
		}

		if lineLen+1+len(word) > width {
			result.WriteString("\n// ")
			result.WriteString(word)
			lineLen = len(word)
		} else {
			result.WriteString(" ")
			result.WriteString(word)
			lineLen += 1 + len(word)
		}
	}

	return result.String()
}

// sanitizeFieldName ensures a field name is a valid Go identifier.
func sanitizeFieldName(name string) string {
	// Replace invalid characters
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Ensure it doesn't start with a digit
	if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
		name = "_" + name
	}

	// Handle Go keywords
	if isGoKeyword(name) {
		name = name + "_"
	}

	return name
}

// isGoKeyword returns true if the string is a Go keyword.
func isGoKeyword(s string) bool {
	keywords := map[string]bool{
		"break": true, "case": true, "chan": true, "const": true,
		"continue": true, "default": true, "defer": true, "else": true,
		"fallthrough": true, "for": true, "func": true, "go": true,
		"goto": true, "if": true, "import": true, "interface": true,
		"map": true, "package": true, "range": true, "return": true,
		"select": true, "struct": true, "switch": true, "type": true,
		"var": true,
	}
	return keywords[s]
}

