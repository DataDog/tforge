package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DataDog/tforge/config"
)

// mustMap extracts a map[string]any value from a parent map by key.
// It calls t.Fatal with a descriptive message instead of panicking on
// missing keys or type mismatches.
func mustMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("key %q not found in map (keys: %v)", key, keys(m))
	}
	typed, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("key %q: expected map[string]any, got %T", key, v)
	}
	return typed
}

// mustSlice extracts a []any value from a parent map by key.
func mustSlice(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("key %q not found in map (keys: %v)", key, keys(m))
	}
	typed, ok := v.([]any)
	if !ok {
		t.Fatalf("key %q: expected []any, got %T", key, v)
	}
	return typed
}

// mustIndexMap extracts a map[string]any from a slice at the given index.
func mustIndexMap(t *testing.T, s []any, idx int) map[string]any {
	t.Helper()
	if idx >= len(s) {
		t.Fatalf("index %d out of range (len %d)", idx, len(s))
	}
	typed, ok := s[idx].(map[string]any)
	if !ok {
		t.Fatalf("index %d: expected map[string]any, got %T", idx, s[idx])
	}
	return typed
}

// keys returns the keys of a map for diagnostic messages.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestToJSON_NilStack(t *testing.T) {
	_, err := ToJSON(nil, "")
	if err == nil {
		t.Fatal("expected error for nil stack, got nil")
	}
}

func TestToJSON_EmptyStack(t *testing.T) {
	stack := config.NewStack("empty")
	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}
	// An empty stack should produce only the "//" comment metadata block
	// and no terraform, provider, resource, data, module, variable, output, or locals keys.
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// The only key should be "//"
	if len(obj) != 1 {
		t.Errorf("expected 1 top-level key, got %d: %v", len(obj), keys(obj))
	}
	comment, ok := obj["//"]
	if !ok {
		t.Fatal("expected '//' key in output")
	}

	// Verify metadata structure
	commentMap, ok := comment.(map[string]any)
	if !ok {
		t.Fatalf("'//' value: expected map[string]any, got %T", comment)
	}
	metadata, ok := commentMap["metadata"]
	if !ok {
		t.Fatal("expected 'metadata' key inside '//'")
	}
	metaMap, ok := metadata.(map[string]any)
	if !ok {
		t.Fatalf("'metadata' value: expected map[string]any, got %T", metadata)
	}
	if metaMap["tool"] != "tforge" {
		t.Errorf("metadata.tool = %v, want %q", metaMap["tool"], "tforge")
	}
	if metaMap["version"] != DefaultVersion {
		t.Errorf("metadata.version = %v, want %q", metaMap["version"], DefaultVersion)
	}
	if metaMap["stackName"] != "empty" {
		t.Errorf("metadata.stackName = %v, want %q", metaMap["stackName"], "empty")
	}

	// Ensure no infrastructure keys are present
	for _, key := range []string{"terraform", "provider", "resource", "data", "module", "variable", "output", "locals"} {
		if _, exists := obj[key]; exists {
			t.Errorf("unexpected key %q in empty stack output", key)
		}
	}
}

func TestToJSON_ProviderNoAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Source: "hashicorp/aws",
		Config: map[string]any{"region": "us-east-1"},
	})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	providers := mustMap(t, obj, "provider")
	awsList := mustSlice(t, providers, "aws")
	if len(awsList) != 1 {
		t.Fatalf("expected 1 aws provider, got %d", len(awsList))
	}
	awsCfg := mustIndexMap(t, awsList, 0)
	if awsCfg["region"] != "us-east-1" {
		t.Errorf("expected region us-east-1, got %v", awsCfg["region"])
	}
	if _, exists := awsCfg["alias"]; exists {
		t.Error("alias should not be present when empty")
	}
}

func TestToJSON_ProviderWithAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Alias:  "west",
		Config: map[string]any{"region": "us-west-2"},
	})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	awsList := mustSlice(t, mustMap(t, obj, "provider"), "aws")
	awsCfg := mustIndexMap(t, awsList, 0)
	if awsCfg["alias"] != "west" {
		t.Errorf("expected alias west, got %v", awsCfg["alias"])
	}
}

func TestToJSON_ProviderWithConfig(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config: map[string]any{
			"region":  "eu-west-1",
			"profile": "dev",
		},
	})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	awsList := mustSlice(t, mustMap(t, obj, "provider"), "aws")
	awsCfg := mustIndexMap(t, awsList, 0)
	if awsCfg["region"] != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %v", awsCfg["region"])
	}
	if awsCfg["profile"] != "dev" {
		t.Errorf("expected profile dev, got %v", awsCfg["profile"])
	}
}

func TestToJSON_ResourceBasic(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123", "instance_type": "t3.micro"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	if res["ami"] != "ami-123" {
		t.Errorf("expected ami ami-123, got %v", res["ami"])
	}
}

func TestToJSON_ResourceMetadataComment(t *testing.T) {
	stack := config.NewStack("my-stack")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	meta := metadataCommentFor(t, stack, "resource", "aws_instance", "web")
	require.Equal(t, "my-stack/web", meta["path"])
	require.Equal(t, "web", meta["uniqueId"])
}

// TestToJSON_ResourceMetadataPath_MatchesCdktf verifies that metadata.path
// uses cdktf's exact shape: forward slashes in the caller-supplied name are
// replaced with `--` (cdktf's construct-tree separator convention), but
// other characters like `{`, `}`, `:` are preserved. uniqueId is the
// SanitizeName'd identifier used as the Terraform resource key.
func TestToJSON_ResourceMetadataPath_MatchesCdktf(t *testing.T) {
	stack := config.NewStack("cdktf_monitoring")
	const rawName = "monitor_api/v2/users/{id}_GET"
	const pathified = "monitor_api--v2--users--{id}_GET"
	const sanitized = "monitor_api--v2--users--id_GET"
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "datadog_monitor",
		Name:       rawName,
		Attributes: map[string]any{"name": "test"},
	}))

	meta := metadataCommentFor(t, stack, "resource", "datadog_monitor", sanitized)
	require.Equal(t, "cdktf_monitoring/"+pathified, meta["path"])
	require.Equal(t, sanitized, meta["uniqueId"])
}

func TestToJSON_DataSourceMetadataComment(t *testing.T) {
	stack := config.NewStack("my-stack")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "ubuntu",
		Attributes: map[string]any{"most_recent": true},
	}))

	meta := metadataCommentFor(t, stack, "data", "aws_ami", "ubuntu")
	require.Equal(t, "my-stack/ubuntu", meta["path"])
	require.Equal(t, "ubuntu", meta["uniqueId"])
}

func TestToJSON_ModuleMetadataComment(t *testing.T) {
	stack := config.NewStack("my-stack")
	require.NoError(t, stack.AddModule(config.Module{
		Name:      "vpc",
		Source:    "./modules/vpc",
		Variables: map[string]any{"cidr": "10.0.0.0/16"},
	}))

	meta := metadataCommentFor(t, stack, "module", "", "vpc")
	require.Equal(t, "my-stack/vpc", meta["path"])
	require.Equal(t, "vpc", meta["uniqueId"])
}

// metadataCommentFor renders stack to JSON and extracts the per-resource
// "//.metadata" block at the given location. For resources/data sources pass
// (kind, type, name) — e.g. ("resource", "aws_instance", "web"); for modules
// the type is unused so pass ("module", "", name).
func metadataCommentFor(t *testing.T, stack *config.Stack, kind, typ, name string) map[string]any {
	t.Helper()

	data, err := ToJSON(stack, "")
	require.NoError(t, err)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	var entry map[string]any
	if kind == "module" {
		entry = mustMap(t, mustMap(t, obj, "module"), name)
	} else {
		entry = mustMap(t, mustMap(t, mustMap(t, obj, kind), typ), name)
	}
	return mustMap(t, mustMap(t, entry, "//"), "metadata")
}

func TestToJSON_ResourceProvider(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Provider:   "aws.west",
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	if res["provider"] != "aws.west" {
		t.Errorf("expected provider aws.west, got %v", res["provider"])
	}
}

func TestToJSON_ResourceProviderAbsentWhenEmpty(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	if _, exists := res["provider"]; exists {
		t.Error("provider key should not be present when Provider is empty")
	}
}

func TestToJSON_ResourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	deps := mustSlice(t, res, "depends_on")
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_ResourceCount(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			Count: 3,
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	// JSON numbers unmarshal as float64
	if res["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", res["count"])
	}
}

func TestToJSON_ResourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]any{"a": "1", "b": "2"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	forEach := mustMap(t, res, "for_each")
	if forEach["a"] != "1" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestToJSON_ResourceLifecycle(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			CreateBeforeDestroy: true,
			PreventDestroy:      true,
			IgnoreChanges:       []string{"tags"},
			ReplaceTriggeredBy:  []string{"null_resource.trigger"},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	if lc["create_before_destroy"] != true {
		t.Error("expected create_before_destroy true")
	}
	if lc["prevent_destroy"] != true {
		t.Error("expected prevent_destroy true")
	}
	ignoreChanges := mustSlice(t, lc, "ignore_changes")
	if len(ignoreChanges) != 1 || ignoreChanges[0] != "tags" {
		t.Errorf("unexpected ignore_changes: %v", ignoreChanges)
	}
}

func TestToJSON_ResourceTimeouts(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Timeouts: &config.Timeouts{
			Create: "30m",
			Delete: "10m",
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	timeouts := mustMap(t, res, "timeouts")
	if timeouts["create"] != "30m" {
		t.Errorf("expected create 30m, got %v", timeouts["create"])
	}
	if timeouts["delete"] != "10m" {
		t.Errorf("expected delete 10m, got %v", timeouts["delete"])
	}
	if _, ok := timeouts["read"]; ok {
		t.Error("read should not be present when empty")
	}
}

func TestToJSON_ResourceProvisioners(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Provisioners: []config.Provisioner{
			{
				Type:      "local-exec",
				Config:    map[string]any{"command": "echo hello"},
				When:      config.WhenCreate,
				OnFailure: config.OnFailureContinue,
			},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	provs := mustSlice(t, res, "provisioner")
	if len(provs) != 1 {
		t.Fatalf("expected 1 provisioner, got %d", len(provs))
	}
	localExec := mustMap(t, mustIndexMap(t, provs, 0), "local-exec")
	if localExec["command"] != "echo hello" {
		t.Errorf("expected command 'echo hello', got %v", localExec["command"])
	}
	if localExec["when"] != "create" {
		t.Errorf("expected when 'create', got %v", localExec["when"])
	}
	if localExec["on_failure"] != "continue" {
		t.Errorf("expected on_failure 'continue', got %v", localExec["on_failure"])
	}
}

func TestToJSON_MultipleResourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-web"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "api",
		Attributes: map[string]any{"ami": "ami-api"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	instances := mustMap(t, mustMap(t, obj, "resource"), "aws_instance")

	web := mustMap(t, instances, "web")
	if web["ami"] != "ami-web" {
		t.Errorf("expected web ami 'ami-web', got %v", web["ami"])
	}

	api := mustMap(t, instances, "api")
	if api["ami"] != "ami-api" {
		t.Errorf("expected api ami 'ami-api', got %v", api["ami"])
	}
}

func TestToJSON_MultipleResourceTypes(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_vpc",
		Name:       "main",
		Attributes: map[string]any{"cidr_block": "10.0.0.0/16"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddResource(&config.Resource{
		Type:       "aws_subnet",
		Name:       "public",
		Attributes: map[string]any{"cidr_block": "10.0.1.0/24"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	resources := mustMap(t, obj, "resource")
	if _, ok := resources["aws_instance"]; !ok {
		t.Error("missing aws_instance resource type")
	}
	if _, ok := resources["aws_vpc"]; !ok {
		t.Error("missing aws_vpc resource type")
	}
	if _, ok := resources["aws_subnet"]; !ok {
		t.Error("missing aws_subnet resource type")
	}
}

func TestToJSON_MultipleDataSourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "ubuntu",
		Attributes: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "amazon_linux",
		Attributes: map[string]any{"most_recent": false},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	amis := mustMap(t, mustMap(t, obj, "data"), "aws_ami")

	ubuntu := mustMap(t, amis, "ubuntu")
	if ubuntu["most_recent"] != true {
		t.Errorf("expected ubuntu most_recent true, got %v", ubuntu["most_recent"])
	}

	amazonLinux := mustMap(t, amis, "amazon_linux")
	if amazonLinux["most_recent"] != false {
		t.Errorf("expected amazon_linux most_recent false, got %v", amazonLinux["most_recent"])
	}
}

func TestToJSON_DataSource(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	if ds["most_recent"] != true {
		t.Errorf("expected most_recent true, got %v", ds["most_recent"])
	}
}

func TestToJSON_DataSourceProvider(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Provider:   "aws.west",
		Attributes: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	if ds["provider"] != "aws.west" {
		t.Errorf("expected provider aws.west, got %v", ds["provider"])
	}
}

func TestToJSON_DataSourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	deps := mustSlice(t, ds, "depends_on")
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_DataSourceCount(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			Count: 3,
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	// JSON numbers unmarshal as float64
	if ds["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", ds["count"])
	}
}

func TestToJSON_DataSourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]any{"a": "1", "b": "2"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	forEach := mustMap(t, ds, "for_each")
	if forEach["a"] != "1" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestToJSON_Variables(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddVariable(config.Variable{
		Name:        "instance_type",
		Type:        "string",
		Description: "The instance type",
		Default:     "t3.micro",
		Sensitive:   true,
		Nullable:    true,
		Validation: &config.VariableValidation{
			Condition:    "length(var.instance_type) > 0",
			ErrorMessage: "Must not be empty",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	v := mustMap(t, mustMap(t, obj, "variable"), "instance_type")
	if v["type"] != "string" {
		t.Errorf("expected type string, got %v", v["type"])
	}
	if v["description"] != "The instance type" {
		t.Errorf("unexpected description: %v", v["description"])
	}
	if v["default"] != "t3.micro" {
		t.Errorf("unexpected default: %v", v["default"])
	}
	if v["sensitive"] != true {
		t.Error("expected sensitive true")
	}
	if v["nullable"] != true {
		t.Error("expected nullable true")
	}
	validation := mustMap(t, v, "validation")
	if validation["condition"] != "length(var.instance_type) > 0" {
		t.Errorf("unexpected condition: %v", validation["condition"])
	}
	if validation["error_message"] != "Must not be empty" {
		t.Errorf("unexpected error_message: %v", validation["error_message"])
	}
}

func TestToJSON_Outputs(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddOutput(config.Output{
		Name:        "instance_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP",
		Sensitive:   true,
		DependsOn:   []string{"aws_instance.web"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	o := mustMap(t, mustMap(t, obj, "output"), "instance_ip")
	if o["value"] != "${aws_instance.web.public_ip}" {
		t.Errorf("unexpected value: %v", o["value"])
	}
	if o["description"] != "The public IP" {
		t.Errorf("unexpected description: %v", o["description"])
	}
	if o["sensitive"] != true {
		t.Error("expected sensitive true")
	}
	deps := mustSlice(t, o, "depends_on")
	if len(deps) != 1 || deps[0] != "aws_instance.web" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
}

func TestToJSON_Locals(t *testing.T) {
	stack := config.NewStack("test")
	stack.SetLocal("env", "production")
	stack.SetLocal("tags", map[string]string{"team": "platform"})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	locals := mustMap(t, obj, "locals")
	if locals["env"] != "production" {
		t.Errorf("unexpected env: %v", locals["env"])
	}
	tags := mustMap(t, locals, "tags")
	if tags["team"] != "platform" {
		t.Errorf("unexpected tags: %v", tags)
	}
}

func TestToJSON_Backend(t *testing.T) {
	stack := config.NewStack("test")
	stack.SetBackend(config.Backend{
		Type: "s3",
		Config: map[string]any{
			"bucket": "my-state",
			"key":    "terraform.tfstate",
			"region": "us-east-1",
		},
	})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	tf := mustMap(t, obj, "terraform")
	backend := mustMap(t, tf, "backend")
	s3 := mustMap(t, backend, "s3")
	if s3["bucket"] != "my-state" {
		t.Errorf("unexpected bucket: %v", s3["bucket"])
	}
}

func TestToJSON_RequiredProviders(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
	})

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	tf := mustMap(t, obj, "terraform")
	rp := mustMap(t, tf, "required_providers")
	aws := mustMap(t, rp, "aws")
	if aws["source"] != "hashicorp/aws" {
		t.Errorf("unexpected source: %v", aws["source"])
	}
	if aws["version"] != "~> 5.0" {
		t.Errorf("unexpected version: %v", aws["version"])
	}
}

func TestToJSON_FullStack(t *testing.T) {
	stack := config.NewStack("full")

	stack.SetBackend(config.Backend{
		Type:   "s3",
		Config: map[string]any{"bucket": "state"},
	})

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
		Config:  map[string]any{"region": "us-east-1"},
	})

	if err := stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{"most_recent": true},
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddVariable(config.Variable{
		Name:    "env",
		Type:    "string",
		Default: "dev",
	}); err != nil {
		t.Fatal(err)
	}

	if err := stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
	}); err != nil {
		t.Fatal(err)
	}

	stack.SetLocal("name", "my-app")

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("output is not valid JSON")
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify all top-level keys are present
	for _, key := range []string{"terraform", "provider", "resource", "data", "variable", "output", "locals"} {
		if _, ok := obj[key]; !ok {
			t.Errorf("missing top-level key: %s", key)
		}
	}
}

func TestToFile(t *testing.T) {
	stack := config.NewStack("file-test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Config: map[string]any{"region": "us-east-1"},
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf.json")

	if err := ToFile(stack, path, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	expected, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	if string(fileData) != string(expected) {
		t.Error("file contents do not match ToJSON output")
	}
}

func TestToFile_CreatesDirectory(t *testing.T) {
	stack := config.NewStack("test")

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "main.tf.json")

	if err := ToFile(stack, path, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}

func TestToJSON_ResourceConnection(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type:    "ssh",
			Host:    "1.2.3.4",
			User:    "admin",
			Port:    22,
			Timeout: "5m",
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "admin" {
		t.Errorf("expected user admin, got %v", conn["user"])
	}
	if conn["port"] != float64(22) {
		t.Errorf("expected port 22, got %v", conn["port"])
	}
	if conn["timeout"] != "5m" {
		t.Errorf("expected timeout 5m, got %v", conn["timeout"])
	}
}

func TestToJSON_ResourceConnectionWithExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type: "winrm",
			Host: "10.0.0.1",
			Extra: map[string]any{
				"https":    true,
				"cacert":   "/path/to/ca.pem",
				"insecure": true,
			},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "winrm" {
		t.Errorf("expected type winrm, got %v", conn["type"])
	}
	if conn["https"] != true {
		t.Errorf("expected https true, got %v", conn["https"])
	}
	if conn["cacert"] != "/path/to/ca.pem" {
		t.Errorf("expected cacert, got %v", conn["cacert"])
	}
}

func TestToJSON_LifecyclePrecondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			Precondition: &config.Condition{
				Condition:    "var.x > 0",
				ErrorMessage: "x must be positive",
			},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	pre := mustMap(t, lc, "precondition")
	if pre["condition"] != "var.x > 0" {
		t.Errorf("expected condition 'var.x > 0', got %v", pre["condition"])
	}
	if pre["error_message"] != "x must be positive" {
		t.Errorf("expected error_message 'x must be positive', got %v", pre["error_message"])
	}
}

func TestToJSON_LifecyclePostcondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			Postcondition: &config.Condition{
				Condition:    "self.public_ip != \"\"",
				ErrorMessage: "instance must have a public IP",
			},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	post := mustMap(t, lc, "postcondition")
	if post["condition"] != "self.public_ip != \"\"" {
		t.Errorf("expected condition, got %v", post["condition"])
	}
	if post["error_message"] != "instance must have a public IP" {
		t.Errorf("expected error_message, got %v", post["error_message"])
	}
}

func TestToJSON_OutputPrecondition(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
		Precondition: &config.Condition{
			Condition:    "aws_instance.web.public_ip != \"\"",
			ErrorMessage: "instance must have a public IP",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	o := mustMap(t, mustMap(t, obj, "output"), "ip")
	pre := mustMap(t, o, "precondition")
	if pre["condition"] != "aws_instance.web.public_ip != \"\"" {
		t.Errorf("expected condition, got %v", pre["condition"])
	}
	if pre["error_message"] != "instance must have a public IP" {
		t.Errorf("expected error_message, got %v", pre["error_message"])
	}
}

func TestToJSON_ProvisionerConnection(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Provisioners: []config.Provisioner{
			{
				Type:   "remote-exec",
				Config: map[string]any{"inline": []string{"echo hello"}},
				Connection: &config.Connection{
					Type:       "ssh",
					Host:       "10.0.0.1",
					User:       "deploy",
					PrivateKey: "file(\"/path/to/key\")",
				},
			},
		},
		Attributes: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	provs := mustSlice(t, res, "provisioner")
	remoteExec := mustMap(t, mustIndexMap(t, provs, 0), "remote-exec")
	conn := mustMap(t, remoteExec, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1, got %v", conn["host"])
	}
	if conn["user"] != "deploy" {
		t.Errorf("expected user deploy, got %v", conn["user"])
	}
	if conn["private_key"] != "file(\"/path/to/key\")" {
		t.Errorf("expected private_key, got %v", conn["private_key"])
	}
}

func TestToJSON_ModuleBasic(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:    "eks",
		Source:  "terraform-aws-modules/eks/aws",
		Version: "~> 19.0",
		Variables: map[string]any{
			"cluster_name":    "my-eks",
			"cluster_version": "1.27",
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := mustMap(t, mustMap(t, obj, "module"), "eks")
	if mod["source"] != "terraform-aws-modules/eks/aws" {
		t.Errorf("expected source terraform-aws-modules/eks/aws, got %v", mod["source"])
	}
	if mod["version"] != "~> 19.0" {
		t.Errorf("expected version ~> 19.0, got %v", mod["version"])
	}
	if mod["cluster_name"] != "my-eks" {
		t.Errorf("expected cluster_name my-eks, got %v", mod["cluster_name"])
	}
	if mod["cluster_version"] != "1.27" {
		t.Errorf("expected cluster_version 1.27, got %v", mod["cluster_version"])
	}
}

func TestToJSON_ModuleWithProviders(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:   "vpc",
		Source: "terraform-aws-modules/vpc/aws",
		Providers: map[string]string{
			"aws": "aws.west",
		},
		Variables: map[string]any{"cidr": "10.0.0.0/16"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := mustMap(t, mustMap(t, obj, "module"), "vpc")
	providers := mustMap(t, mod, "providers")
	if providers["aws"] != "aws.west" {
		t.Errorf("expected providers.aws = aws.west, got %v", providers["aws"])
	}
}

func TestToJSON_ModuleWithMetaArguments(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:      "server",
		Source:    "./modules/server",
		Variables: map[string]any{"name": "web"},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
			Count:     3,
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := mustMap(t, mustMap(t, obj, "module"), "server")
	if mod["source"] != "./modules/server" {
		t.Errorf("expected source ./modules/server, got %v", mod["source"])
	}
	deps := mustSlice(t, mod, "depends_on")
	if len(deps) != 1 || deps[0] != "aws_vpc.main" {
		t.Errorf("unexpected depends_on: %v", deps)
	}
	if mod["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", mod["count"])
	}
	// version should not be present for local modules
	if _, ok := mod["version"]; ok {
		t.Error("version should not be present when empty")
	}
}

func TestToJSON_ModuleForEach(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddModule(config.Module{
		Name:      "bucket",
		Source:    "terraform-aws-modules/s3-bucket/aws",
		Variables: map[string]any{"bucket": "${each.key}-bucket"},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]string{"logs": "logs", "data": "data"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}

	mod := mustMap(t, mustMap(t, obj, "module"), "bucket")
	forEach := mustMap(t, mod, "for_each")
	if forEach["logs"] != "logs" {
		t.Errorf("unexpected for_each: %v", forEach)
	}
}

func TestConnectionJSON_ExtraDoesNotOverwriteKnownFields(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			Extra: map[string]any{
				"type": "winrm",
				"host": "evil.example.com",
			},
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected known field type=ssh to take precedence, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected known field host=1.2.3.4 to take precedence, got %v", conn["host"])
	}
}

func TestConnectionJSON_NilExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			User: "admin",
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "admin" {
		t.Errorf("expected user admin, got %v", conn["user"])
	}
}

func TestConnectionJSON_EmptyExtra(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type:  "ssh",
			Host:  "1.2.3.4",
			Extra: map[string]any{},
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
}

func TestConnectionJSON_ExtraOnlyUnknownFields(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			User: "deploy",
			Extra: map[string]any{
				"bastion_host": "10.0.0.1",
				"bastion_user": "bastion",
			},
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := ToJSON(stack, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	if conn["type"] != "ssh" {
		t.Errorf("expected type ssh, got %v", conn["type"])
	}
	if conn["host"] != "1.2.3.4" {
		t.Errorf("expected host 1.2.3.4, got %v", conn["host"])
	}
	if conn["user"] != "deploy" {
		t.Errorf("expected user deploy, got %v", conn["user"])
	}
	if conn["bastion_host"] != "10.0.0.1" {
		t.Errorf("expected bastion_host 10.0.0.1, got %v", conn["bastion_host"])
	}
	if conn["bastion_user"] != "bastion" {
		t.Errorf("expected bastion_user bastion, got %v", conn["bastion_user"])
	}
}

func TestResourceConfigJSON_MarshalJSON(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		rc := resourceConfigJSON{
			Config:    map[string]any{"existing": "value"},
			Provider:  "aws.west",
			DependsOn: []string{"aws_vpc.main"},
			Count:     3,
			ForEach:   map[string]string{"a": "1"},
		}

		data, err := json.Marshal(rc)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		if m["provider"] != "aws.west" {
			t.Errorf("provider = %v, want aws.west", m["provider"])
		}
		deps, ok := m["depends_on"].([]any)
		if !ok {
			t.Fatalf("depends_on: expected []any, got %T", m["depends_on"])
		}
		if len(deps) != 1 || deps[0] != "aws_vpc.main" {
			t.Errorf("depends_on = %v, want [aws_vpc.main]", deps)
		}
		if m["count"] != float64(3) {
			t.Errorf("count = %v, want 3", m["count"])
		}
		fe, ok := m["for_each"].(map[string]any)
		if !ok {
			t.Fatalf("for_each: expected map[string]any, got %T", m["for_each"])
		}
		if fe["a"] != "1" {
			t.Errorf("for_each = %v, want map[a:1]", fe)
		}
		if m["existing"] != "value" {
			t.Errorf("existing = %v, want value", m["existing"])
		}
	})

	t.Run("no meta fields set", func(t *testing.T) {
		rc := resourceConfigJSON{
			Config: map[string]any{"existing": "value"},
		}

		data, err := json.Marshal(rc)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		for _, key := range []string{"provider", "depends_on", "count", "for_each", "lifecycle", "timeouts", "connection", "provisioner"} {
			if _, ok := m[key]; ok {
				t.Errorf("%s should not be present", key)
			}
		}
		if m["existing"] != "value" {
			t.Errorf("existing = %v, want value", m["existing"])
		}
	})
}

func TestProvisionerJSON_MarshalJSON(t *testing.T) {
	p := provisionerJSON{
		Type:      "local-exec",
		Config:    map[string]any{"command": "echo hello"},
		When:      "create",
		OnFailure: "continue",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	inner, ok := m["local-exec"].(map[string]any)
	if !ok {
		t.Fatalf("expected local-exec key, got keys: %v", keys(m))
	}
	if inner["command"] != "echo hello" {
		t.Errorf("command = %v, want echo hello", inner["command"])
	}
	if inner["when"] != "create" {
		t.Errorf("when = %v, want create", inner["when"])
	}
	if inner["on_failure"] != "continue" {
		t.Errorf("on_failure = %v, want continue", inner["on_failure"])
	}
}

func TestProviderConfigJSON_MarshalJSON(t *testing.T) {
	t.Run("with alias", func(t *testing.T) {
		p := providerConfigJSON{
			Config: map[string]any{"region": "us-east-1"},
			Alias:  "east",
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["region"] != "us-east-1" {
			t.Errorf("region = %v, want us-east-1", m["region"])
		}
		if m["alias"] != "east" {
			t.Errorf("alias = %v, want east", m["alias"])
		}
	})

	t.Run("without alias", func(t *testing.T) {
		p := providerConfigJSON{
			Config: map[string]any{"region": "us-west-2"},
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if _, ok := m["alias"]; ok {
			t.Error("alias should not be present")
		}
	})
}

// typedMonitor mimics a generated typed resource payload: schema-named fields
// via JSON tags so render-time marshaling produces the correct attribute keys.
type typedMonitor struct {
	Name *string `json:"name,omitempty"`
	Type *string `json:"type,omitempty"`
}

func strPtr(s string) *string { return &s }

func TestToJSON_TypedAttributes(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{Name: "datadog"})
	if err := stack.AddResource(&config.Resource{
		Type:            "datadog_monitor",
		Name:            "cpu",
		TypedAttributes: &typedMonitor{Name: strPtr("hi"), Type: strPtr("metric alert")},
	}); err != nil {
		t.Fatalf("AddResourcePtr error = %v", err)
	}

	data, err := ToJSON(stack, "test")
	if err != nil {
		t.Fatalf("ToJSON error = %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	cpu := mustMap(t, mustMap(t, mustMap(t, out, "resource"), "datadog_monitor"), "cpu")
	if cpu["name"] != "hi" {
		t.Errorf("name = %v, want \"hi\"", cpu["name"])
	}
	if cpu["type"] != "metric alert" {
		t.Errorf("type = %v, want \"metric alert\"", cpu["type"])
	}
}

func TestToJSON_TypedAttributes_MutatesAfterAdd(t *testing.T) {
	stack := config.NewStack("test")
	payload := &typedMonitor{Name: strPtr("initial")}
	if err := stack.AddResource(&config.Resource{
		Type:            "datadog_monitor",
		Name:            "cpu",
		TypedAttributes: payload,
	}); err != nil {
		t.Fatalf("AddResourcePtr error = %v", err)
	}

	// Mutate the payload after adding — render must see the new value.
	payload.Name = strPtr("updated")

	data, err := ToJSON(stack, "test")
	if err != nil {
		t.Fatalf("ToJSON error = %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	cpu := mustMap(t, mustMap(t, mustMap(t, out, "resource"), "datadog_monitor"), "cpu")
	if cpu["name"] != "updated" {
		t.Errorf("post-mutation name = %v, want \"updated\"", cpu["name"])
	}
}

func TestToJSON_BothAttributesAndTypedAttributes_Error(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:            "datadog_monitor",
		Name:            "cpu",
		Attributes:      map[string]any{"name": "untyped"},
		TypedAttributes: &typedMonitor{Name: strPtr("typed")},
	}); err != nil {
		t.Fatalf("AddResourcePtr error = %v", err)
	}

	if _, err := ToJSON(stack, "test"); err == nil {
		t.Fatal("ToJSON should error when both Attributes and TypedAttributes are set")
	}
}

// An empty (but non-nil) Attributes map alongside TypedAttributes is still
// an explicit double-assignment by the caller and must be flagged.
func TestToJSON_EmptyAttributesAndTypedAttributes_Error(t *testing.T) {
	stack := config.NewStack("test")
	if err := stack.AddResource(&config.Resource{
		Type:            "datadog_monitor",
		Name:            "cpu",
		Attributes:      map[string]any{},
		TypedAttributes: &typedMonitor{Name: strPtr("typed")},
	}); err != nil {
		t.Fatalf("AddResourcePtr error = %v", err)
	}

	if _, err := ToJSON(stack, "test"); err == nil {
		t.Fatal("ToJSON should error even when Attributes is an empty (but non-nil) map")
	}
}
