package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if schema.Provider == nil {
		t.Fatal("schema.Provider is nil")
	}
	if schema.Provider.ResourceSchemas == nil {
		t.Fatal("ResourceSchemas is nil")
	}
	rs, ok := schema.Provider.ResourceSchemas["test_thing"]
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

	if err := SaveSchemaToFile(original, path); err != nil {
		t.Fatalf("SaveSchemaToFile: %v", err)
	}

	loaded, err := LoadSchemaFromFile(path)
	if err != nil {
		t.Fatalf("LoadSchemaFromFile: %v", err)
	}

	if loaded.Provider == nil {
		t.Fatal("loaded Provider is nil")
	}
	rs, ok := loaded.Provider.ResourceSchemas["test_resource"]
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

func TestCacheDirDefault(t *testing.T) {
	// Unset env var to test default
	orig := os.Getenv("TFORGE_CACHE_DIR")
	os.Unsetenv("TFORGE_CACHE_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("TFORGE_CACHE_DIR", orig)
		}
	}()

	dir := CacheDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".tforge", "cache")
	if dir != expected {
		t.Errorf("CacheDir() = %q, want %q", dir, expected)
	}
}

func TestCacheDirEnvOverride(t *testing.T) {
	orig := os.Getenv("TFORGE_CACHE_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("TFORGE_CACHE_DIR", orig)
		} else {
			os.Unsetenv("TFORGE_CACHE_DIR")
		}
	}()

	os.Setenv("TFORGE_CACHE_DIR", "/custom/cache")
	dir := CacheDir()
	if dir != "/custom/cache" {
		t.Errorf("CacheDir() = %q, want %q", dir, "/custom/cache")
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
	if got := err.Error(); !contains(got, "not found") {
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

// helper for substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// listVersionsFromRegistry tests
// ---------------------------------------------------------------------------

func TestListVersionsFromRegistry_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/providers/datadog/datadog/versions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"versions": [{"version": "3.39.0"}, {"version": "3.38.0"}]}`)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	versions, err := listVersionsFromRegistry(context.Background(), srv.URL, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0] != "3.39.0" {
		t.Errorf("versions[0] = %q, want %q", versions[0], "3.39.0")
	}
	if versions[1] != "3.38.0" {
		t.Errorf("versions[1] = %q, want %q", versions[1], "3.38.0")
	}
}

func TestListVersionsFromRegistry_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := Source{Namespace: "nonexistent", Name: "provider"}
	_, err := listVersionsFromRegistry(context.Background(), srv.URL, src)
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestListVersionsFromRegistry_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := listVersionsFromRegistry(context.Background(), srv.URL, src)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if errors.Is(err, ErrProviderNotFound) {
		t.Error("should not be ErrProviderNotFound for 500")
	}
}

func TestListVersionsFromRegistry_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{not valid json}`)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := listVersionsFromRegistry(context.Background(), srv.URL, src)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestListVersionsFromRegistry_EmptyVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"versions": []}`)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	versions, err := listVersionsFromRegistry(context.Background(), srv.URL, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

// ---------------------------------------------------------------------------
// getProviderDownloadURL tests
// ---------------------------------------------------------------------------

func TestGetProviderDownloadURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"download_url": "https://example.com/file.zip"}`)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	url, err := getProviderDownloadURL(context.Background(), srv.URL, src, "3.39.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/file.zip" {
		t.Errorf("download URL = %q, want %q", url, "https://example.com/file.zip")
	}
}

func TestGetProviderDownloadURL_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := getProviderDownloadURL(context.Background(), srv.URL, src, "99.99.99")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestGetProviderDownloadURL_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := getProviderDownloadURL(context.Background(), srv.URL, src, "3.39.0")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if errors.Is(err, ErrProviderNotFound) {
		t.Error("should not be ErrProviderNotFound for 500")
	}
}

func TestGetProviderDownloadURL_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json at all`)
	}))
	defer srv.Close()

	src := Source{Namespace: "datadog", Name: "datadog"}
	_, err := getProviderDownloadURL(context.Background(), srv.URL, src, "3.39.0")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// SaveSchemaToFile tests
// ---------------------------------------------------------------------------

func TestSaveSchemaToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "schema.json")

	schema := &Schema{
		Source:  Source{Namespace: "test", Name: "test"},
		Version: "2.0.0",
		Provider: &ProviderSchema{
			ConfigSchema: &SchemaBlock{
				Block: &BlockType{
					Attributes: map[string]*Attribute{
						"api_key": {Type: json.RawMessage(`"string"`), Required: true},
					},
				},
			},
			ResourceSchemas: map[string]*SchemaBlock{
				"test_resource": {
					Block: &BlockType{
						Attributes: map[string]*Attribute{
							"id":   {Type: json.RawMessage(`"string"`), Computed: true},
							"name": {Type: json.RawMessage(`"string"`), Required: true},
						},
					},
				},
			},
			DataSourceSchemas: map[string]*SchemaBlock{
				"test_data": {
					Block: &BlockType{
						Attributes: map[string]*Attribute{
							"filter": {Type: json.RawMessage(`"string"`), Optional: true},
						},
					},
				},
			},
		},
	}

	if err := SaveSchemaToFile(schema, path); err != nil {
		t.Fatalf("SaveSchemaToFile: %v", err)
	}

	// Verify the file exists and contains valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	var ps ProviderSchema
	if err := json.Unmarshal(data, &ps); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}

	// Verify all sections are present
	if ps.ConfigSchema == nil {
		t.Error("ConfigSchema missing from saved file")
	}
	if _, ok := ps.ResourceSchemas["test_resource"]; !ok {
		t.Error("test_resource missing from saved file")
	}
	if _, ok := ps.DataSourceSchemas["test_data"]; !ok {
		t.Error("test_data missing from saved file")
	}
}

func TestSaveSchemaToFile_BadPath(t *testing.T) {
	schema := &Schema{
		Provider: &ProviderSchema{},
	}
	err := SaveSchemaToFile(schema, "/nonexistent/dir/schema.json")
	if err == nil {
		t.Error("expected error writing to nonexistent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// CacheDir default path check
// ---------------------------------------------------------------------------

func TestCacheDirDefaultContainsTforgeCache(t *testing.T) {
	orig := os.Getenv("TFORGE_CACHE_DIR")
	os.Unsetenv("TFORGE_CACHE_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("TFORGE_CACHE_DIR", orig)
		}
	}()

	dir := CacheDir()
	suffix := filepath.Join(".tforge", "cache")
	if !strings.HasSuffix(dir, suffix) {
		t.Errorf("CacheDir() = %q, want it to end with %q", dir, suffix)
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
	if schema.Provider.ConfigSchema == nil {
		t.Error("ConfigSchema is nil")
	}
	if _, ok := schema.Provider.ResourceSchemas["r1"]; !ok {
		t.Error("r1 resource not found")
	}
	if _, ok := schema.Provider.DataSourceSchemas["d1"]; !ok {
		t.Error("d1 data source not found")
	}
}

func TestLoadSchemaFromJSON_EmptyObject(t *testing.T) {
	schema, err := LoadSchemaFromJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.Provider == nil {
		t.Fatal("Provider is nil")
	}
}

// ---------------------------------------------------------------------------
// FetchSchemaJSON — just verify invalid source is caught
// ---------------------------------------------------------------------------

func TestFetchSchemaJSON_InvalidSource(t *testing.T) {
	_, err := FetchSchemaJSON("invalid", "1.0.0")
	if err == nil {
		t.Error("expected error for invalid source, got nil")
	}
}

// ---------------------------------------------------------------------------
// ListVersions — invalid source
// ---------------------------------------------------------------------------

func TestListVersions_InvalidSource(t *testing.T) {
	_, err := ListVersions(context.Background(), "badsource")
	if err == nil {
		t.Error("expected error for invalid source, got nil")
	}
}

// ---------------------------------------------------------------------------
// DownloadProvider — invalid source
// ---------------------------------------------------------------------------

func TestDownloadProvider_InvalidSource(t *testing.T) {
	_, err := DownloadProvider(context.Background(), "badsource", "1.0.0")
	if err == nil {
		t.Error("expected error for invalid source, got nil")
	}
}
