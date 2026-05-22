package config

import (
	"regexp"
	"strings"
)

// disallowedNameChars matches any character that is not valid in a Terraform
// identifier. Terraform requires names to match `[a-zA-Z_][a-zA-Z0-9_-]*`;
// this regex captures everything outside the body charset (the leading-char
// rule is left to the caller — sanitization yields a body-compatible string).
var disallowedNameChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// SanitizeName transforms an arbitrary string into a Terraform-compatible
// identifier body. Forward slashes are replaced with `--` to preserve visual
// segmentation when names are built from path-like inputs (e.g. HTTP routes
// such as `api/v2/foo` → `api--v2--foo`), and any remaining disallowed
// characters are stripped.
//
// This intentionally matches cdktf's character rule
// (`s.replace(/[^A-Za-z0-9_-]/g, "")` in `packages/cdktf/lib/private/unique.ts`).
// Terraform itself accepts Unicode letters in identifiers, so the ASCII-only
// rule does silently drop non-ASCII letters (e.g. `café` → `caf`); we keep
// that behavior on purpose so generated names stay portable across providers
// and match what cdktf-derived pipelines produce. Callers that need Unicode
// names must rename their inputs before construction.
//
// SanitizeName is idempotent: applying it to its own output is a no-op.
//
// Example:
//
//	SanitizeName("api/v2/{path:path}_GET") == "api--v2--pathpath_GET"
func SanitizeName(s string) string {
	s = strings.ReplaceAll(s, "/", "--")
	return disallowedNameChars.ReplaceAllString(s, "")
}
