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

const (
	dirPerm  = os.FileMode(0755)
	filePerm = os.FileMode(0644)
)

// WriteFile writes a jennifer file to disk, creating directories as needed.
func WriteFile(f *jen.File, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
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
		return fmt.Errorf("formatting generated code: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, formatted, filePerm); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}

	return nil
}

// toGoName converts a snake_case Terraform name to PascalCase Go name.
// If the result would start with a digit (invalid Go identifier), it is
// prefixed with "X". An empty input returns an empty string.
func (g *Generator) toGoName(tfName string) string {
	parts := strings.Split(tfName, "_")
	var result strings.Builder

	for _, part := range parts {
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		if g.initialisms[upper] {
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

// DefaultInitialisms is the default set of terms that are kept fully
// uppercased in generated Go identifiers (e.g. "api_key" → APIKey).
//
// The base list is Go's canonical set of initialisms, sourced from the
// now-archived golang.org/x/lint module. That list was never exported as
// a reusable API (golang/lint#272 requested it, but the repo was frozen
// before it happened). Its successor, staticcheck ST1003, expanded the
// list and made it configurable, but still does not export it as a
// library. There is no shared Go package for this — every code generator
// (gqlgen, oapi-codegen, sqlc, etc.) vendors its own copy.
//
// On top of the golang.org/x/lint entries we add a small set of terms
// commonly found in Terraform provider schemas (IDS, VPC, ARN, IAM, …).
// Users can fully override this list via Config.Initialisms.
var DefaultInitialisms = []string{
	// Go canonical initialisms (from golang.org/x/lint, frozen 2021-05)
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID",
	"HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "LHS", "QPS",
	"RAM", "RHS", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP",
	"TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI", "URL",
	"UTF8", "VM", "XML", "XMPP", "XSRF", "XSS",
	// Terraform / cloud-provider terms
	"ARN", "EC2", "ECS", "EKS", "IAM", "IDS", "KMS",
	"OS", "RDS", "S3", "SNS", "SQS", "SSL", "VPC",
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
