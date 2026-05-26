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

	"github.com/stretchr/testify/require"
)

func TestParseSourceValid(t *testing.T) {
	tests := []struct {
		input     string
		hostname  string
		namespace string
		name      string
	}{
		// Default registry (two-segment).
		{"datadog/datadog", "", "datadog", "datadog"},
		{"hashicorp/aws", "", "hashicorp", "aws"},

		// Hostname-qualified (three-segment): explicit registry.
		{"registry.opentofu.org/hashicorp/aws", "registry.opentofu.org", "hashicorp", "aws"},
		{"registry.terraform.io/datadog/datadog", "registry.terraform.io", "datadog", "datadog"},
		{"mirror.corp.internal/team/myprovider", "mirror.corp.internal", "team", "myprovider"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := ParseSource(tt.input)
			require.NoError(t, err, "ParseSource(%q) returned error", tt.input)
			require.Equal(t, tt.hostname, src.Hostname, "Hostname")
			require.Equal(t, tt.namespace, src.Namespace, "Namespace")
			require.Equal(t, tt.name, src.Name, "Name")
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
		{"empty namespace (2-part)", "/name"},
		{"empty name (2-part)", "ns/"},
		{"empty hostname (3-part)", "/ns/name"},
		{"empty namespace (3-part)", "host//name"},
		{"empty name (3-part)", "host/ns/"},
		{"four segments", "a/b/c/d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			require.Error(t, err, "ParseSource(%q) expected error, got nil", tt.input)
		})
	}
}

func TestSourceString(t *testing.T) {
	t.Run("default registry (no hostname)", func(t *testing.T) {
		src := Source{Namespace: "dd", Name: "dd"}
		require.Equal(t, "dd/dd", src.String())
	})

	t.Run("hostname-qualified", func(t *testing.T) {
		src := Source{Hostname: "registry.opentofu.org", Namespace: "hashicorp", Name: "aws"}
		require.Equal(t, "registry.opentofu.org/hashicorp/aws", src.String())
	})

	t.Run("round-trip ParseSource -> String", func(t *testing.T) {
		for _, addr := range []string{
			"datadog/datadog",
			"hashicorp/aws",
			"registry.opentofu.org/hashicorp/aws",
			"mirror.corp.internal/team/myprovider",
		} {
			src, err := ParseSource(addr)
			require.NoError(t, err)
			require.Equal(t, addr, src.String(), "round-trip mismatch for %q", addr)
		}
	})
}

func TestLoadSchemaFromJSONValid(t *testing.T) {
	validJSON := []byte(`{"resource_schemas":{"test_thing":{"block":{"attributes":{"id":{"type":"string","computed":true}}}}}}`)
	schema, err := LoadSchemaFromJSON(validJSON)
	require.NoError(t, err, "LoadSchemaFromJSON returned error")
	require.NotNil(t, schema, "LoadSchemaFromJSON returned nil schema")
	require.NotNil(t, schema.ResourceSchemas, "ResourceSchemas is nil")
	rs, ok := schema.ResourceSchemas["test_thing"]
	require.True(t, ok, "test_thing resource not found")
	require.NotNil(t, rs.Block, "test_thing block is nil")
	attr, ok := rs.Block.Attributes["id"]
	require.True(t, ok, "id attribute not found")
	require.True(t, attr.Computed, "expected id to be computed")
}

func TestLoadSchemaFromJSONInvalid(t *testing.T) {
	_, err := LoadSchemaFromJSON([]byte(`{not json`))
	require.Error(t, err, "expected error for invalid JSON, got nil")
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
	require.NoError(t, err, "MarshalIndent")
	require.NoError(t, os.WriteFile(path, data, 0644), "WriteFile")

	loaded, err := LoadSchemaFromFile(path)
	require.NoError(t, err, "LoadSchemaFromFile")
	require.NotNil(t, loaded, "loaded schema is nil")
	rs, ok := loaded.ResourceSchemas["test_resource"]
	require.True(t, ok, "test_resource not found after roundtrip")
	require.NotNil(t, rs.Block, "block is nil after roundtrip")
	require.Equal(t, "a resource", rs.Block.Description)
	attr, ok := rs.Block.Attributes["name"]
	require.True(t, ok, "name attribute not found after roundtrip")
	require.True(t, attr.Required, "expected name to be required after roundtrip")
}

func TestLoadSchemaFromFileNotFound(t *testing.T) {
	_, err := LoadSchemaFromFile("/nonexistent/path/schema.json")
	require.Error(t, err, "expected error for nonexistent file, got nil")
}

// ---------------------------------------------------------------------------
// writeTerraformConfig tests
// ---------------------------------------------------------------------------

func TestWriteTerraformConfig_PreservesHostname(t *testing.T) {
	tests := []struct {
		name     string
		src      Source
		version  string
		wantLine string
	}{
		{
			name:     "default registry omits hostname",
			src:      Source{Namespace: "datadog", Name: "datadog"},
			version:  "3.50.0",
			wantLine: `source  = "datadog/datadog"`,
		},
		{
			name:     "hostname-qualified source",
			src:      Source{Hostname: "registry.opentofu.org", Namespace: "hashicorp", Name: "aws"},
			version:  "5.0.0",
			wantLine: `source  = "registry.opentofu.org/hashicorp/aws"`,
		},
		{
			name:     "internal mirror",
			src:      Source{Hostname: "mirror.corp.internal", Namespace: "datadog", Name: "datadog"},
			version:  "3.50.0",
			wantLine: `source  = "mirror.corp.internal/datadog/datadog"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, writeTerraformConfig(dir, tt.src, tt.version))
			data, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			require.NoError(t, err)
			require.Contains(t, string(data), tt.wantLine, "main.tf source line missing %q\nfull contents:\n%s", tt.wantLine, string(data))
		})
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
	require.NoError(t, err, "unexpected error")
	require.NotNil(t, ps.ConfigSchema, "ConfigSchema is nil")
	_, ok := ps.ResourceSchemas["datadog_monitor"]
	require.True(t, ok, "expected datadog_monitor in ResourceSchemas")
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
	require.NoError(t, err, "unexpected error")
	_, ok := ps.ResourceSchemas["aws_instance"]
	require.True(t, ok, "expected aws_instance in ResourceSchemas")
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
	require.NoError(t, err, "unexpected error")
	_, ok := ps.ResourceSchemas["mycloud_thing"]
	require.True(t, ok, "expected mycloud_thing in ResourceSchemas")
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
	require.NoError(t, err, "unexpected error")
	_, ok := ps.ResourceSchemas["aws_correct"]
	require.True(t, ok, "expected aws_correct in ResourceSchemas (exact match)")
	_, ok = ps.ResourceSchemas["aws_wrong"]
	require.False(t, ok, "should not have returned hashicorp/aws schema")
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
	require.NoError(t, err, "unexpected error")
	_, ok := ps.ResourceSchemas["aws_instance"]
	require.True(t, ok, "expected aws_instance via suffix fallback")
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
	require.Error(t, err, "expected error for ambiguous suffix match, got nil")
	require.Contains(t, err.Error(), "multiple", "error = %q, want it to mention 'multiple'", err.Error())
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
	require.NoError(t, err, "unexpected error")
	_, ok := ps.ResourceSchemas["datadog_monitor"]
	require.True(t, ok, "expected datadog_monitor from exact match")
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
	require.Error(t, err, "expected error for provider not found, got nil")
	require.Contains(t, err.Error(), "not found", "error = %q, want it to contain 'not found'", err.Error())
}

func TestParseSchemaOutput_EmptyProviders(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{"format_version": "1.0", "provider_schemas": {}}`)

	_, err := parseSchemaOutput(data, src)
	require.Error(t, err, "expected error for empty provider_schemas, got nil")
}

func TestParseSchemaOutput_InvalidJSON(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := parseSchemaOutput([]byte(`{not json`), src)
	require.Error(t, err, "expected error for invalid JSON, got nil")
}

func TestParseSchemaOutput_RejectsSubstringFalsePositive(t *testing.T) {
	// "datadog/datadog" must NOT match a key whose name segment merely starts
	// with "datadog" — the previous strings.Contains check matched any key
	// that happened to embed the source string as a substring.
	src := Source{Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/datadog/datadogextra": {
				"provider": {"block": {}},
				"resource_schemas": {"datadogextra_thing": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	_, err := parseSchemaOutput(data, src)
	require.Error(t, err, "expected error: datadog/datadog must not match datadog/datadogextra")
}

func TestParseSchemaOutput_HostnameQualifiedRequiresHostnameMatch(t *testing.T) {
	// A hostname-pinned src must never resolve to a provider served from a
	// different registry, even when namespace and name happen to align.
	src := Source{Hostname: "registry.opentofu.org", Namespace: "datadog", Name: "datadog"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/datadog/datadog": {
				"provider": {"block": {}},
				"resource_schemas": {"datadog_monitor": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	_, err := parseSchemaOutput(data, src)
	require.Error(t, err, "expected error: hostname-pinned src must not fall back to a different registry")
}

func TestParseSchemaOutput_HostnameQualifiedExactMatch(t *testing.T) {
	src := Source{Hostname: "registry.opentofu.org", Namespace: "hashicorp", Name: "aws"}
	data := []byte(`{
		"format_version": "1.0",
		"provider_schemas": {
			"registry.terraform.io/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_wrong": {"block": {}}},
				"data_source_schemas": {}
			},
			"registry.opentofu.org/hashicorp/aws": {
				"provider": {"block": {}},
				"resource_schemas": {"aws_correct": {"block": {}}},
				"data_source_schemas": {}
			}
		}
	}`)

	ps, err := parseSchemaOutput(data, src)
	require.NoError(t, err)
	_, ok := ps.ResourceSchemas["aws_correct"]
	require.True(t, ok, "expected aws_correct from the requested registry")
	_, ok = ps.ResourceSchemas["aws_wrong"]
	require.False(t, ok, "must not return the same provider from a different registry")
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
	require.NoError(t, err, "unexpected error")
	require.NotNil(t, schema.ConfigSchema, "ConfigSchema is nil")
	_, ok := schema.ResourceSchemas["r1"]
	require.True(t, ok, "r1 resource not found")
	_, ok = schema.DataSourceSchemas["d1"]
	require.True(t, ok, "d1 data source not found")
}

func TestLoadSchemaFromJSON_EmptyObject(t *testing.T) {
	schema, err := LoadSchemaFromJSON([]byte(`{}`))
	require.NoError(t, err, "unexpected error")
	require.NotNil(t, schema, "schema is nil")
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
			require.Len(t, got, len(tt.want), "filterEnv() returned %d entries, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			for i := range got {
				require.Equal(t, tt.want[i], got[i], "filterEnv()[%d] = %q, want %q", i, got[i], tt.want[i])
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
	require.NoError(t, err, "findCLI() with TFORGE_CLI=echo returned error")
	require.NotEmpty(t, path, "findCLI() returned empty path")
}

func TestFindCLI_ForcedInvalid(t *testing.T) {
	t.Setenv("TFORGE_CLI", "nonexistent_binary_xyz")
	_, err := findCLI()
	require.Error(t, err, "findCLI() with TFORGE_CLI=nonexistent_binary_xyz expected error, got nil")
	require.Contains(t, err.Error(), "TFORGE_CLI", "error = %q, want it to mention TFORGE_CLI", err.Error())
}

// ---------------------------------------------------------------------------
// Context cancellation tests
// ---------------------------------------------------------------------------

func TestFetchSchema_InitFailure(t *testing.T) {
	skipIfNoCLI(t)

	// Use a provider that does not exist in any registry so that init fails.
	_, err := FetchSchema(context.Background(), "nonexistent-namespace/nonexistent-provider-xyz", "0.0.1")
	require.Error(t, err, "expected error for nonexistent provider, got nil")
	// The error must mention "init failed" and include the CLI output.
	require.Contains(t, err.Error(), "init failed", "error = %q, want it to contain 'init failed'", err.Error())
}

func TestFetchSchema_NoCLI(t *testing.T) {
	// Force a binary that doesn't exist via TFORGE_CLI to trigger the
	// ErrCLINotFound-like path without affecting the real PATH.
	t.Setenv("TFORGE_CLI", "nonexistent_binary_xyz_tforge")

	_, err := FetchSchema(context.Background(), "hashicorp/null", "3.2.1")
	require.Error(t, err, "expected error when CLI is not found, got nil")
	require.Contains(t, err.Error(), "not found", "error = %q, want it to mention 'not found'", err.Error())
}

func TestFetchSchema_InvalidSource(t *testing.T) {
	// Invalid source should fail at parse stage before any CLI work.
	_, err := FetchSchema(context.Background(), "no-slash", "1.0.0")
	require.Error(t, err, "expected error for invalid source, got nil")
	require.Contains(t, err.Error(), "invalid provider source", "error = %q, want it to mention 'invalid provider source'", err.Error())
}

func TestFetchSchema_CancelledContext(t *testing.T) {
	skipIfNoCLI(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	start := time.Now()
	_, err := FetchSchema(ctx, "hashicorp/null", "3.2.1")
	elapsed := time.Since(start)

	require.Error(t, err, "expected error for cancelled context, got nil")
	// The error should originate from the cancelled context propagating
	// through exec.CommandContext. It should not hang.
	require.True(t,
		strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "signal: killed"),
		"expected context cancellation error, got: %v", err)
	// Should return quickly (well under 10 seconds), not hang waiting for init.
	require.Less(t, elapsed, 10*time.Second, "FetchSchema with cancelled context took %v, expected fast return", elapsed)
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

	require.Error(t, err, "expected error for expired context, got nil")
	require.True(t,
		strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "signal: killed"),
		"expected deadline exceeded error, got: %v", err)
	require.Less(t, elapsed, 10*time.Second, "FetchSchema with expired context took %v, expected fast return", elapsed)
}

func TestFetchSchema_InvalidSourceWithCancelledContext(t *testing.T) {
	// Even with a cancelled context, invalid source should fail fast
	// at the parse stage (before any CLI commands).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchSchema(ctx, "invalid", "1.0.0")
	require.Error(t, err, "expected error for invalid source, got nil")
	require.Contains(t, err.Error(), "invalid provider source", "expected parse error, got: %v", err)
}
