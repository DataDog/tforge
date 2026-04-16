package provider

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSourceValid(t *testing.T) {
	tests := []struct {
		input     string
		namespace string
		name      string
	}{
		{"datadog/datadog", "datadog", "datadog"},
		{"hashicorp/aws", "hashicorp", "aws"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) returned error: %v", tt.input, err)
			}
			if src.Namespace != tt.namespace {
				t.Errorf("Namespace = %q, want %q", src.Namespace, tt.namespace)
			}
			if src.Name != tt.name {
				t.Errorf("Name = %q, want %q", src.Name, tt.name)
			}
		})
	}
}

func TestParseSourceInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no slash", "invalid"},
		{"empty string", ""},
		{"empty namespace", "/name"},
		{"empty name", "ns/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if err == nil {
				t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestSourceString(t *testing.T) {
	src := Source{Namespace: "dd", Name: "dd"}
	if got := src.String(); got != "dd/dd" {
		t.Errorf("String() = %q, want %q", got, "dd/dd")
	}
}

func TestLoadSchemaFromJSONValid(t *testing.T) {
	validJSON := []byte(`{"resource_schemas":{"test_thing":{"block":{"attributes":{"id":{"type":"string","computed":true}}}}}}`)
	schema, err := LoadSchemaFromJSON(validJSON)
	if err != nil {
		t.Fatalf("LoadSchemaFromJSON returned error: %v", err)
	}
	if schema == nil {
		t.Fatal("LoadSchemaFromJSON returned nil schema")
	}
	if schema.ResourceSchemas == nil {
		t.Fatal("ResourceSchemas is nil")
	}
	rs, ok := schema.ResourceSchemas["test_thing"]
	if !ok {
		t.Fatal("test_thing resource not found")
	}
	if rs.Block == nil {
		t.Fatal("test_thing block is nil")
	}
	attr, ok := rs.Block.Attributes["id"]
	if !ok {
		t.Fatal("id attribute not found")
	}
	if !attr.Computed {
		t.Error("expected id to be computed")
	}
}

func TestLoadSchemaFromJSONInvalid(t *testing.T) {
	_, err := LoadSchemaFromJSON([]byte(`{not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSaveAndLoadSchemaRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "schema.json")

	original := &Schema{
		Source:  Source{Namespace: "test", Name: "test"},
		Version: "1.0.0",
		Provider: &ProviderSchema{
			ResourceSchemas: map[string]*SchemaBlock{
				"test_resource": {
					Block: &BlockType{
						Attributes: map[string]*Attribute{
							"name": {
								Type:     []byte(`"string"`),
								Required: true,
							},
						},
						Description: "a resource",
					},
				},
			},
		},
	}

	// Marshal and write manually (SaveSchemaToFile was removed as unused)
	data, err := json.MarshalIndent(original.Provider, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadSchemaFromFile(path)
	if err != nil {
		t.Fatalf("LoadSchemaFromFile: %v", err)
	}

	if loaded == nil {
		t.Fatal("loaded schema is nil")
	}
	rs, ok := loaded.ResourceSchemas["test_resource"]
	if !ok {
		t.Fatal("test_resource not found after roundtrip")
	}
	if rs.Block == nil {
		t.Fatal("block is nil after roundtrip")
	}
	if rs.Block.Description != "a resource" {
		t.Errorf("description = %q, want %q", rs.Block.Description, "a resource")
	}
	attr, ok := rs.Block.Attributes["name"]
	if !ok {
		t.Fatal("name attribute not found after roundtrip")
	}
	if !attr.Required {
		t.Error("expected name to be required after roundtrip")
	}
}

func TestLoadSchemaFromFileNotFound(t *testing.T) {
	_, err := LoadSchemaFromFile("/nonexistent/path/schema.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// ---------------------------------------------------------------------------
// parseSchemaOutput tests
// ---------------------------------------------------------------------------

func TestParseSchemaOutput_ExactMatch(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.opentofu.org/datadog/datadog": {
				"provider": {"block": {"attributes": {"api_key": {"type": "string"}}}},
				"resource_schemas": {
					"datadog_monitor": {"block": {"attributes": {"id": {"type": "string", "computed": true}}}}
				},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.ConfigSchema == nil {
		t.Fatal("ConfigSchema is nil")
	}
	if _, ok := ps.ResourceSchemas["datadog_monitor"]; !ok {
		t.Error("expected datadog_monitor in ResourceSchemas")
	}
}

func TestParseSchemaOutput_ContainsSrcString(t *testing.T) {
	src := Source{Namespace: "hashicorp", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_instance": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["aws_instance"]; !ok {
		t.Error("expected aws_instance in ResourceSchemas")
	}
}

func TestParseSchemaOutput_SuffixMatch(t *testing.T) {
	// Key has suffix "/name" but doesn't contain full "namespace/name"
	src := Source{Namespace: "custom", Name: "mycloud"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"localhost/other/mycloud": {
				"provider": {"block": {}},
				"resource_schemas": {"mycloud_thing": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["mycloud_thing"]; !ok {
		t.Error("expected mycloud_thing in ResourceSchemas")
	}
}

func TestParseSchemaOutput_ExactMatchPreferredOverSuffix(t *testing.T) {
	// When both custom/aws and hashicorp/aws exist, requesting custom/aws
	// must return the custom/aws schema, not hashicorp/aws.
	src := Source{Namespace: "custom", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_wrong": {"block": {}}},
				"data_source_schemas": {}
			},
			"registry.terraform.io/custom/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_correct": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["aws_correct"]; !ok {
		t.Error("expected aws_correct in ResourceSchemas (exact match)")
	}
	if _, ok := ps.ResourceSchemas["aws_wrong"]; ok {
		t.Error("should not have returned hashicorp/aws schema")
	}
}

func TestParseSchemaOutput_SuffixMatchRejectsWrongNamespace(t *testing.T) {
	// When requesting custom/aws but only hashicorp/aws exists, the suffix
	// match should still work (single candidate) but not silently if there
	// are multiple candidates with different namespaces.
	src := Source{Namespace: "custom", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_instance": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	// Single suffix candidate: falls back gracefully.
	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["aws_instance"]; !ok {
		t.Error("expected aws_instance via suffix fallback")
	}
}

func TestParseSchemaOutput_AmbiguousSuffixReturnsError(t *testing.T) {
	// When two providers share the same name but different namespaces and
	// the exact match doesn't hit, the function should return an error
	// instead of non-deterministically picking one.
	src := Source{Namespace: "unrelated", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_a": {"block": {}}},
				"data_source_schemas": {}
			},
			"registry.terraform.io/custom/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_b": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	_, err := parseSchemaOutput(data, src)
	if err == nil {
		t.Fatal("expected error for ambiguous suffix match, got nil")
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("error = %q, want it to mention 'multiple'", err.Error())
	}
}

func TestParseSchemaOutput_CrossRegistryExactMatch(t *testing.T) {
	// Exact match across different registries.
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.opentofu.org/datadog/datadog": {
				"provider": {"block": {}},
				"resource_schemas": {"datadog_monitor": {"block": {}}},
				"data_source_schemas": {}
			},
			"registry.terraform.io/other/datadog": {
				"provider": {"block": {}},
				"resource_schemas": {"other_datadog_thing": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ps.ResourceSchemas["datadog_monitor"]; !ok {
		t.Error("expected datadog_monitor from exact match")
	}
}

func TestParseSchemaOutput_ProviderNotFound(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.opentofu.org/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {},
				"data_source_schemas": {}
			}
		}
	}`)

	_, err := parseSchemaOutput(data, src)
	if err == nil {
		t.Fatal("expected error for provider not found, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", got)
	}
}

func TestParseSchemaOutput_EmptyProviders(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{"format_version": "1.0", "provider_schemas": {}}`)

	_, err := parseSchemaOutput(data, src)
	if err == nil {
		t.Fatal("expected error for empty provider_schemas, got nil")
	}
}

func TestParseSchemaOutput_InvalidJSON(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := parseSchemaOutput([]byte(`{not json`), src)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// LoadSchemaFromJSON edge cases
// ---------------------------------------------------------------------------

func TestLoadSchemaFromJSON_WithAllSections(t *testing.T) {
	input := `{
		"provider": {"block": {"attributes": {"key": {"type": "string"}}}},
		"resource_schemas": {"r1": {"block": {"attributes": {"id": {"type": "string", "computed": true}}}}},
		"data_source_schemas": {"d1": {"block": {"attributes": {"filter": {"type": "string", "optional": true}}}}}
	}`

	schema, err := LoadSchemaFromJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.ConfigSchema == nil {
		t.Error("ConfigSchema is nil")
	}
	if _, ok := schema.ResourceSchemas["r1"]; !ok {
		t.Error("r1 resource not found")
	}
	if _, ok := schema.DataSourceSchemas["d1"]; !ok {
		t.Error("d1 data source not found")
	}
}

func TestLoadSchemaFromJSON_EmptyObject(t *testing.T) {
	schema, err := LoadSchemaFromJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema == nil {
		t.Fatal("schema is nil")
	}
}

// ---------------------------------------------------------------------------
// Context cancellation tests
// ---------------------------------------------------------------------------

// skipIfNoCLI skips the test if neither tofu nor terraform is in PATH.
func skipIfNoCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tofu"); err == nil {
		return
	}
	if _, err := exec.LookPath("terraform"); err == nil {
		return
	}
	t.Skip("skipping: neither tofu nor terraform found in PATH")
}

// ---------------------------------------------------------------------------
// filterEnv tests
// ---------------------------------------------------------------------------

func TestFilterEnv(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		keys []string
		want []string
	}{
		{
			name: "remove single key",
			env:  []string{"FOO=1", "BAR=2", "BAZ=3"},
			keys: []string{"BAR"},
			want: []string{"FOO=1", "BAZ=3"},
		},
		{
			name: "remove multiple keys",
			env:  []string{"FOO=1", "BAR=2", "BAZ=3"},
			keys: []string{"FOO", "BAZ"},
			want: []string{"BAR=2"},
		},
		{
			name: "no matches keeps all",
			env:  []string{"FOO=1", "BAR=2"},
			keys: []string{"NOPE"},
			want: []string{"FOO=1", "BAR=2"},
		},
		{
			name: "empty env",
			env:  []string{},
			keys: []string{"FOO"},
			want: []string{},
		},
		{
			name: "key is prefix of another key",
			env:  []string{"FOO=1", "FOO_BAR=2"},
			keys: []string{"FOO"},
			want: []string{"FOO_BAR=2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEnv(tt.env, tt.keys...)
			if len(got) != len(tt.want) {
				t.Fatalf("filterEnv() returned %d entries, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("filterEnv()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findCLI tests
// ---------------------------------------------------------------------------

func TestFindCLI_ForcedValid(t *testing.T) {
	t.Setenv("TFORGE_CLI", "echo")
	path, err := findCLI()
	if err != nil {
		t.Fatalf("findCLI() with TFORGE_CLI=echo returned error: %v", err)
	}
	if path == "" {
		t.Error("findCLI() returned empty path")
	}
}

func TestFindCLI_ForcedInvalid(t *testing.T) {
	t.Setenv("TFORGE_CLI", "nonexistent_binary_xyz")
	_, err := findCLI()
	if err == nil {
		t.Fatal("findCLI() with TFORGE_CLI=nonexistent_binary_xyz expected error, got nil")
	}
	if !strings.Contains(err.Error(), "TFORGE_CLI") {
		t.Errorf("error = %q, want it to mention TFORGE_CLI", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Context cancellation tests
// ---------------------------------------------------------------------------

func TestFetchSchema_InitFailure(t *testing.T) {
	skipIfNoCLI(t)

	// Use a provider that does not exist in any registry so that init fails.
	_, err := FetchSchema(context.Background(), "nonexistent-namespace/nonexistent-provider-xyz", "0.0.1")
	if err == nil {
		t.Fatal("expected error for nonexistent provider, got nil")
	}
	// The error must mention "init failed" and include the CLI output.
	if !strings.Contains(err.Error(), "init failed") {
		t.Errorf("error = %q, want it to contain 'init failed'", err.Error())
	}
}

func TestFetchSchema_NoCLI(t *testing.T) {
	// Force a binary that doesn't exist via TFORGE_CLI to trigger the
	// ErrCLINotFound-like path without affecting the real PATH.
	t.Setenv("TFORGE_CLI", "nonexistent_binary_xyz_tforge")

	_, err := FetchSchema(context.Background(), "hashicorp/null", "3.2.1")
	if err == nil {
		t.Fatal("expected error when CLI is not found, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err.Error())
	}
}

func TestFetchSchema_InvalidSource(t *testing.T) {
	// Invalid source should fail at parse stage before any CLI work.
	_, err := FetchSchema(context.Background(), "no-slash", "1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid source, got nil")
	}
	if !strings.Contains(err.Error(), "invalid provider source") {
		t.Errorf("error = %q, want it to mention 'invalid provider source'", err.Error())
	}
}

func TestFetchSchema_CancelledContext(t *testing.T) {
	skipIfNoCLI(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	start := time.Now()
	_, err := FetchSchema(ctx, "hashicorp/null", "3.2.1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	// The error should originate from the cancelled context propagating
	// through exec.CommandContext. It should not hang.
	if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("expected context cancellation error, got: %v", err)
	}
	// Should return quickly (well under 10 seconds), not hang waiting for init.
	if elapsed > 10*time.Second {
		t.Errorf("FetchSchema with cancelled context took %v, expected fast return", elapsed)
	}
}

func TestFetchSchema_DeadlineExceeded(t *testing.T) {
	skipIfNoCLI(t)

	// Use a very short timeout so the init command cannot complete.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the timeout a moment to expire before calling, to ensure
	// the context is already expired when exec.CommandContext runs.
	time.Sleep(5 * time.Millisecond)

	start := time.Now()
	_, err := FetchSchema(ctx, "hashicorp/null", "3.2.1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for expired context, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("expected deadline exceeded error, got: %v", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("FetchSchema with expired context took %v, expected fast return", elapsed)
	}
}

func TestFetchSchema_InvalidSourceWithCancelledContext(t *testing.T) {
	// Even with a cancelled context, invalid source should fail fast
	// at the parse stage (before any CLI commands).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchSchema(ctx, "invalid", "1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid source, got nil")
	}
	if !strings.Contains(err.Error(), "invalid provider source") {
		t.Errorf("expected parse error, got: %v", err)
	}
}
