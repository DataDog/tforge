package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DataDog/tforge"
	"github.com/DataDog/tforge/config"
)

// defaultVersion is the value tforge.Version compiles to without any link-time
// override. Kept in tests so the version-stamping assertions don't accidentally
// shadow drift in the default.
const defaultVersion = "dev"

// mustMap extracts a map[string]any value from a parent map by key.
// It calls t.Fatal with a descriptive message instead of panicking on
// missing keys or type mismatches.
func mustMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key]
	require.True(t, ok, "key %q not found in map (keys: %v)", key, keys(m))
	typed, ok := v.(map[string]any)
	require.True(t, ok, "key %q: expected map[string]any, got %T", key, v)
	return typed
}

// mustSlice extracts a []any value from a parent map by key.
func mustSlice(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	v, ok := m[key]
	require.True(t, ok, "key %q not found in map (keys: %v)", key, keys(m))
	typed, ok := v.([]any)
	require.True(t, ok, "key %q: expected []any, got %T", key, v)
	return typed
}

// mustIndexMap extracts a map[string]any from a slice at the given index.
func mustIndexMap(t *testing.T, s []any, idx int) map[string]any {
	t.Helper()
	require.Less(t, idx, len(s), "index %d out of range (len %d)", idx, len(s))
	typed, ok := s[idx].(map[string]any)
	require.True(t, ok, "index %d: expected map[string]any, got %T", idx, s[idx])
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
	_, err := ToJSON(nil)
	require.Error(t, err)
}

// TestToJSON_VersionStamp verifies the value of tforge.Version is what gets
// stamped into the JSON metadata. Mirrors how consumers wire the package's
// pinned version through the linker.
func TestToJSON_VersionStamp(t *testing.T) {
	original := tforge.Version
	t.Cleanup(func() { tforge.Version = original })

	const wantPinned = "v1.2.3-pinned"
	tforge.Version = wantPinned

	data, err := ToJSON(config.NewStack("test"))
	require.NoError(t, err)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	meta := obj["//"].(map[string]any)["metadata"].(map[string]any)
	require.Equal(t, wantPinned, meta["version"],
		"tforge.Version should be stamped into //.metadata.version verbatim")
}

func TestToJSON_EmptyStack(t *testing.T) {
	stack := config.NewStack("empty")
	data, err := ToJSON(stack)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "output is not valid JSON")
	// An empty stack should produce only the "//" comment metadata block
	// and no terraform, provider, resource, data, module, variable, output, or locals keys.
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "failed to unmarshal")

	// The only key should be "//"
	require.Len(t, obj, 1, "expected 1 top-level key, got %d: %v", len(obj), keys(obj))
	comment, ok := obj["//"]
	require.True(t, ok, "expected '//' key in output")

	// Verify metadata structure
	commentMap, ok := comment.(map[string]any)
	require.True(t, ok, "'//' value: expected map[string]any, got %T", comment)
	metadata, ok := commentMap["metadata"]
	require.True(t, ok, "expected 'metadata' key inside '//'")
	metaMap, ok := metadata.(map[string]any)
	require.True(t, ok, "'metadata' value: expected map[string]any, got %T", metadata)
	require.Equal(t, "tforge", metaMap["tool"], "metadata.tool = %v, want %q", metaMap["tool"], "tforge")
	require.Equal(t, defaultVersion, metaMap["version"], "metadata.version = %v, want %q", metaMap["version"], defaultVersion)
	require.Equal(t, "empty", metaMap["stackName"], "metadata.stackName = %v, want %q", metaMap["stackName"], "empty")

	// Ensure no infrastructure keys are present
	for _, key := range []string{"terraform", "provider", "resource", "data", "module", "variable", "output", "locals"} {
		_, exists := obj[key]
		require.False(t, exists, "unexpected key %q in empty stack output", key)
	}
}

func TestToJSON_ProviderNoAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Source: "hashicorp/aws",
		Config: map[string]any{"region": "us-east-1"},
	})

	data, err := ToJSON(stack)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "output is not valid JSON")

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal")

	providers := mustMap(t, obj, "provider")
	awsList := mustSlice(t, providers, "aws")
	require.Len(t, awsList, 1, "expected 1 aws provider, got %d", len(awsList))
	awsCfg := mustIndexMap(t, awsList, 0)
	require.Equal(t, "us-east-1", awsCfg["region"], "expected region us-east-1, got %v", awsCfg["region"])
	_, exists := awsCfg["alias"]
	require.False(t, exists, "alias should not be present when empty")
}

func TestToJSON_ProviderWithAlias(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:   "aws",
		Alias:  "west",
		Config: map[string]any{"region": "us-west-2"},
	})

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal")

	awsList := mustSlice(t, mustMap(t, obj, "provider"), "aws")
	awsCfg := mustIndexMap(t, awsList, 0)
	require.Equal(t, "west", awsCfg["alias"], "expected alias west, got %v", awsCfg["alias"])
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

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal")

	awsList := mustSlice(t, mustMap(t, obj, "provider"), "aws")
	awsCfg := mustIndexMap(t, awsList, 0)
	require.Equal(t, "eu-west-1", awsCfg["region"], "expected region eu-west-1, got %v", awsCfg["region"])
	require.Equal(t, "dev", awsCfg["profile"], "expected profile dev, got %v", awsCfg["profile"])
}

func TestToJSON_ResourceBasic(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123", "instance_type": "t3.micro"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal")

	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	require.Equal(t, "ami-123", res["ami"], "expected ami ami-123, got %v", res["ami"])
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

	data, err := ToJSON(stack)
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
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Provider:   "aws.west",
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	require.Equal(t, "aws.west", res["provider"], "expected provider aws.west, got %v", res["provider"])
}

func TestToJSON_ResourceProviderAbsentWhenEmpty(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	_, exists := res["provider"]
	require.False(t, exists, "provider key should not be present when Provider is empty")
}

func TestToJSON_ResourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	deps := mustSlice(t, res, "depends_on")
	require.Len(t, deps, 1)
	require.Equal(t, "aws_vpc.main", deps[0], "unexpected depends_on: %v", deps)
}

func TestToJSON_ResourceCount(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			Count: 3,
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	// JSON numbers unmarshal as float64
	require.Equal(t, float64(3), res["count"], "expected count 3, got %v", res["count"])
}

func TestToJSON_ResourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]any{"a": "1", "b": "2"},
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	forEach := mustMap(t, res, "for_each")
	require.Equal(t, "1", forEach["a"], "unexpected for_each: %v", forEach)
}

func TestToJSON_ResourceLifecycle(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			CreateBeforeDestroy: true,
			PreventDestroy:      true,
			IgnoreChanges:       []string{"tags"},
			ReplaceTriggeredBy:  []string{"null_resource.trigger"},
		},
		Attributes: map[string]any{},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	require.Equal(t, true, lc["create_before_destroy"], "expected create_before_destroy true")
	require.Equal(t, true, lc["prevent_destroy"], "expected prevent_destroy true")
	ignoreChanges := mustSlice(t, lc, "ignore_changes")
	require.Len(t, ignoreChanges, 1)
	require.Equal(t, "tags", ignoreChanges[0], "unexpected ignore_changes: %v", ignoreChanges)
}

func TestToJSON_ResourceTimeouts(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Timeouts: &config.Timeouts{
			Create: "30m",
			Delete: "10m",
		},
		Attributes: map[string]any{},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	timeouts := mustMap(t, res, "timeouts")
	require.Equal(t, "30m", timeouts["create"], "expected create 30m, got %v", timeouts["create"])
	require.Equal(t, "10m", timeouts["delete"], "expected delete 10m, got %v", timeouts["delete"])
	_, ok := timeouts["read"]
	require.False(t, ok, "read should not be present when empty")
}

func TestToJSON_ResourceProvisioners(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	provs := mustSlice(t, res, "provisioner")
	require.Len(t, provs, 1, "expected 1 provisioner, got %d", len(provs))
	localExec := mustMap(t, mustIndexMap(t, provs, 0), "local-exec")
	require.Equal(t, "echo hello", localExec["command"], "expected command 'echo hello', got %v", localExec["command"])
	require.Equal(t, "create", localExec["when"], "expected when 'create', got %v", localExec["when"])
	require.Equal(t, "continue", localExec["on_failure"], "expected on_failure 'continue', got %v", localExec["on_failure"])
}

func TestToJSON_MultipleResourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-web"},
	}))
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "api",
		Attributes: map[string]any{"ami": "ami-api"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	instances := mustMap(t, mustMap(t, obj, "resource"), "aws_instance")

	web := mustMap(t, instances, "web")
	require.Equal(t, "ami-web", web["ami"], "expected web ami 'ami-web', got %v", web["ami"])

	api := mustMap(t, instances, "api")
	require.Equal(t, "ami-api", api["ami"], "expected api ami 'ami-api', got %v", api["ami"])
}

func TestToJSON_MultipleResourceTypes(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}))
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_vpc",
		Name:       "main",
		Attributes: map[string]any{"cidr_block": "10.0.0.0/16"},
	}))
	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_subnet",
		Name:       "public",
		Attributes: map[string]any{"cidr_block": "10.0.1.0/24"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	resources := mustMap(t, obj, "resource")
	_, ok := resources["aws_instance"]
	require.True(t, ok, "missing aws_instance resource type")
	_, ok = resources["aws_vpc"]
	require.True(t, ok, "missing aws_vpc resource type")
	_, ok = resources["aws_subnet"]
	require.True(t, ok, "missing aws_subnet resource type")
}

func TestToJSON_MultipleDataSourcesSameType(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "ubuntu",
		Attributes: map[string]any{"most_recent": true},
	}))
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "amazon_linux",
		Attributes: map[string]any{"most_recent": false},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	amis := mustMap(t, mustMap(t, obj, "data"), "aws_ami")

	ubuntu := mustMap(t, amis, "ubuntu")
	require.Equal(t, true, ubuntu["most_recent"], "expected ubuntu most_recent true, got %v", ubuntu["most_recent"])

	amazonLinux := mustMap(t, amis, "amazon_linux")
	require.Equal(t, false, amazonLinux["most_recent"], "expected amazon_linux most_recent false, got %v", amazonLinux["most_recent"])
}

func TestToJSON_DataSource(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{"most_recent": true},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	require.Equal(t, true, ds["most_recent"], "expected most_recent true, got %v", ds["most_recent"])
}

func TestToJSON_DataSourceProvider(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Provider:   "aws.west",
		Attributes: map[string]any{"most_recent": true},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	require.Equal(t, "aws.west", ds["provider"], "expected provider aws.west, got %v", ds["provider"])
}

func TestToJSON_DataSourceDependsOn(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	deps := mustSlice(t, ds, "depends_on")
	require.Len(t, deps, 1)
	require.Equal(t, "aws_vpc.main", deps[0], "unexpected depends_on: %v", deps)
}

func TestToJSON_DataSourceCount(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			Count: 3,
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	// JSON numbers unmarshal as float64
	require.Equal(t, float64(3), ds["count"], "expected count 3, got %v", ds["count"])
}

func TestToJSON_DataSourceForEach(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]any{"a": "1", "b": "2"},
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	ds := mustMap(t, mustMap(t, mustMap(t, obj, "data"), "aws_ami"), "latest")
	forEach := mustMap(t, ds, "for_each")
	require.Equal(t, "1", forEach["a"], "unexpected for_each: %v", forEach)
}

func TestToJSON_Variables(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddVariable(config.Variable{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	v := mustMap(t, mustMap(t, obj, "variable"), "instance_type")
	require.Equal(t, "string", v["type"], "expected type string, got %v", v["type"])
	require.Equal(t, "The instance type", v["description"], "unexpected description: %v", v["description"])
	require.Equal(t, "t3.micro", v["default"], "unexpected default: %v", v["default"])
	require.Equal(t, true, v["sensitive"], "expected sensitive true")
	require.Equal(t, true, v["nullable"], "expected nullable true")
	validation := mustMap(t, v, "validation")
	require.Equal(t, "length(var.instance_type) > 0", validation["condition"], "unexpected condition: %v", validation["condition"])
	require.Equal(t, "Must not be empty", validation["error_message"], "unexpected error_message: %v", validation["error_message"])
}

func TestToJSON_Outputs(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddOutput(config.Output{
		Name:        "instance_ip",
		Value:       "${aws_instance.web.public_ip}",
		Description: "The public IP",
		Sensitive:   true,
		DependsOn:   []string{"aws_instance.web"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	o := mustMap(t, mustMap(t, obj, "output"), "instance_ip")
	require.Equal(t, "${aws_instance.web.public_ip}", o["value"], "unexpected value: %v", o["value"])
	require.Equal(t, "The public IP", o["description"], "unexpected description: %v", o["description"])
	require.Equal(t, true, o["sensitive"], "expected sensitive true")
	deps := mustSlice(t, o, "depends_on")
	require.Len(t, deps, 1)
	require.Equal(t, "aws_instance.web", deps[0], "unexpected depends_on: %v", deps)
}

func TestToJSON_Locals(t *testing.T) {
	stack := config.NewStack("test")
	stack.SetLocal("env", "production")
	stack.SetLocal("tags", map[string]string{"team": "platform"})

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	locals := mustMap(t, obj, "locals")
	require.Equal(t, "production", locals["env"], "unexpected env: %v", locals["env"])
	tags := mustMap(t, locals, "tags")
	require.Equal(t, "platform", tags["team"], "unexpected tags: %v", tags)
}

// TestToJSON_RequiredVersion verifies that Stack.RequiredVersion is emitted
// as `terraform.required_version` and that setting it alone is enough to
// materialize the `terraform` block (no backend, no required_providers).
func TestToJSON_RequiredVersion(t *testing.T) {
	stack := config.NewStack("test")
	stack.RequiredVersion = ">= 1.3.7"

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	tf := mustMap(t, obj, "terraform")
	require.Equal(t, ">= 1.3.7", tf["required_version"],
		"required_version not stamped into terraform block")
}

// TestToJSON_NoTerraformBlockWhenEmpty verifies an empty Stack still skips
// the terraform block — RequiredVersion is the only new way to materialize it.
func TestToJSON_NoTerraformBlockWhenEmpty(t *testing.T) {
	data, err := ToJSON(config.NewStack("test"))
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	_, hasTF := obj["terraform"]
	require.False(t, hasTF, "terraform block should be omitted on an empty stack")
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

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	tf := mustMap(t, obj, "terraform")
	backend := mustMap(t, tf, "backend")
	s3 := mustMap(t, backend, "s3")
	require.Equal(t, "my-state", s3["bucket"], "unexpected bucket: %v", s3["bucket"])
}

func TestToJSON_RequiredProviders(t *testing.T) {
	stack := config.NewStack("test")
	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.0",
	})

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	tf := mustMap(t, obj, "terraform")
	rp := mustMap(t, tf, "required_providers")
	aws := mustMap(t, rp, "aws")
	require.Equal(t, "hashicorp/aws", aws["source"], "unexpected source: %v", aws["source"])
	require.Equal(t, "~> 5.0", aws["version"], "unexpected version: %v", aws["version"])
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

	require.NoError(t, stack.AddResource(&config.Resource{
		Type:       "aws_instance",
		Name:       "web",
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	require.NoError(t, stack.AddDataSource(&config.DataSource{
		Type:       "aws_ami",
		Name:       "latest",
		Attributes: map[string]any{"most_recent": true},
	}))

	require.NoError(t, stack.AddVariable(config.Variable{
		Name:    "env",
		Type:    "string",
		Default: "dev",
	}))

	require.NoError(t, stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
	}))

	stack.SetLocal("name", "my-app")

	data, err := ToJSON(stack)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "output is not valid JSON")

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj), "unmarshal")

	// Verify all top-level keys are present
	for _, key := range []string{"terraform", "provider", "resource", "data", "variable", "output", "locals"} {
		_, ok := obj[key]
		require.True(t, ok, "missing top-level key: %s", key)
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

	require.NoError(t, ToFile(stack, path))

	fileData, err := os.ReadFile(path)
	require.NoError(t, err, "reading file")

	expected, err := ToJSON(stack)
	require.NoError(t, err, "ToJSON")

	require.Equal(t, string(expected), string(fileData), "file contents do not match ToJSON output")
}

func TestToFile_CreatesDirectory(t *testing.T) {
	stack := config.NewStack("test")

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "main.tf.json")

	require.NoError(t, ToFile(stack, path))

	_, err := os.Stat(path)
	require.False(t, os.IsNotExist(err), "file was not created")
}

func TestToJSON_ResourceConnection(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "ssh", conn["type"], "expected type ssh, got %v", conn["type"])
	require.Equal(t, "1.2.3.4", conn["host"], "expected host 1.2.3.4, got %v", conn["host"])
	require.Equal(t, "admin", conn["user"], "expected user admin, got %v", conn["user"])
	require.Equal(t, float64(22), conn["port"], "expected port 22, got %v", conn["port"])
	require.Equal(t, "5m", conn["timeout"], "expected timeout 5m, got %v", conn["timeout"])
}

func TestToJSON_ResourceConnectionWithExtra(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "winrm", conn["type"], "expected type winrm, got %v", conn["type"])
	require.Equal(t, true, conn["https"], "expected https true, got %v", conn["https"])
	require.Equal(t, "/path/to/ca.pem", conn["cacert"], "expected cacert, got %v", conn["cacert"])
}

func TestToJSON_LifecyclePrecondition(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			Precondition: &config.Condition{
				Condition:    "var.x > 0",
				ErrorMessage: "x must be positive",
			},
		},
		Attributes: map[string]any{},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	pre := mustMap(t, lc, "precondition")
	require.Equal(t, "var.x > 0", pre["condition"], "expected condition 'var.x > 0', got %v", pre["condition"])
	require.Equal(t, "x must be positive", pre["error_message"], "expected error_message 'x must be positive', got %v", pre["error_message"])
}

func TestToJSON_LifecyclePostcondition(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Lifecycle: &config.Lifecycle{
			Postcondition: &config.Condition{
				Condition:    "self.public_ip != \"\"",
				ErrorMessage: "instance must have a public IP",
			},
		},
		Attributes: map[string]any{},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	lc := mustMap(t, res, "lifecycle")
	post := mustMap(t, lc, "postcondition")
	require.Equal(t, "self.public_ip != \"\"", post["condition"], "expected condition, got %v", post["condition"])
	require.Equal(t, "instance must have a public IP", post["error_message"], "expected error_message, got %v", post["error_message"])
}

func TestToJSON_OutputPrecondition(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddOutput(config.Output{
		Name:  "ip",
		Value: "${aws_instance.web.public_ip}",
		Precondition: &config.Condition{
			Condition:    "aws_instance.web.public_ip != \"\"",
			ErrorMessage: "instance must have a public IP",
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	o := mustMap(t, mustMap(t, obj, "output"), "ip")
	pre := mustMap(t, o, "precondition")
	require.Equal(t, "aws_instance.web.public_ip != \"\"", pre["condition"], "expected condition, got %v", pre["condition"])
	require.Equal(t, "instance must have a public IP", pre["error_message"], "expected error_message, got %v", pre["error_message"])
}

func TestToJSON_ProvisionerConnection(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	provs := mustSlice(t, res, "provisioner")
	remoteExec := mustMap(t, mustIndexMap(t, provs, 0), "remote-exec")
	conn := mustMap(t, remoteExec, "connection")
	require.Equal(t, "ssh", conn["type"], "expected type ssh, got %v", conn["type"])
	require.Equal(t, "10.0.0.1", conn["host"], "expected host 10.0.0.1, got %v", conn["host"])
	require.Equal(t, "deploy", conn["user"], "expected user deploy, got %v", conn["user"])
	require.Equal(t, "file(\"/path/to/key\")", conn["private_key"], "expected private_key, got %v", conn["private_key"])
}

func TestToJSON_ModuleBasic(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddModule(config.Module{
		Name:    "eks",
		Source:  "terraform-aws-modules/eks/aws",
		Version: "~> 19.0",
		Variables: map[string]any{
			"cluster_name":    "my-eks",
			"cluster_version": "1.27",
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	mod := mustMap(t, mustMap(t, obj, "module"), "eks")
	require.Equal(t, "terraform-aws-modules/eks/aws", mod["source"], "expected source terraform-aws-modules/eks/aws, got %v", mod["source"])
	require.Equal(t, "~> 19.0", mod["version"], "expected version ~> 19.0, got %v", mod["version"])
	require.Equal(t, "my-eks", mod["cluster_name"], "expected cluster_name my-eks, got %v", mod["cluster_name"])
	require.Equal(t, "1.27", mod["cluster_version"], "expected cluster_version 1.27, got %v", mod["cluster_version"])
}

func TestToJSON_ModuleWithProviders(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddModule(config.Module{
		Name:   "vpc",
		Source: "terraform-aws-modules/vpc/aws",
		Providers: map[string]string{
			"aws": "aws.west",
		},
		Variables: map[string]any{"cidr": "10.0.0.0/16"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	mod := mustMap(t, mustMap(t, obj, "module"), "vpc")
	providers := mustMap(t, mod, "providers")
	require.Equal(t, "aws.west", providers["aws"], "expected providers.aws = aws.west, got %v", providers["aws"])
}

func TestToJSON_ModuleWithMetaArguments(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddModule(config.Module{
		Name:      "server",
		Source:    "./modules/server",
		Variables: map[string]any{"name": "web"},
		MetaArguments: config.MetaArguments{
			DependsOn: []string{"aws_vpc.main"},
			Count:     3,
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	mod := mustMap(t, mustMap(t, obj, "module"), "server")
	require.Equal(t, "./modules/server", mod["source"], "expected source ./modules/server, got %v", mod["source"])
	deps := mustSlice(t, mod, "depends_on")
	require.Len(t, deps, 1)
	require.Equal(t, "aws_vpc.main", deps[0], "unexpected depends_on: %v", deps)
	require.Equal(t, float64(3), mod["count"], "expected count 3, got %v", mod["count"])
	// version should not be present for local modules
	_, ok := mod["version"]
	require.False(t, ok, "version should not be present when empty")
}

func TestToJSON_ModuleForEach(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddModule(config.Module{
		Name:      "bucket",
		Source:    "terraform-aws-modules/s3-bucket/aws",
		Variables: map[string]any{"bucket": "${each.key}-bucket"},
		MetaArguments: config.MetaArguments{
			ForEach: map[string]string{"logs": "logs", "data": "data"},
		},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))

	mod := mustMap(t, mustMap(t, obj, "module"), "bucket")
	forEach := mustMap(t, mod, "for_each")
	require.Equal(t, "logs", forEach["logs"], "unexpected for_each: %v", forEach)
}

func TestConnectionJSON_ExtraDoesNotOverwriteKnownFields(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "ssh", conn["type"], "expected known field type=ssh to take precedence, got %v", conn["type"])
	require.Equal(t, "1.2.3.4", conn["host"], "expected known field host=1.2.3.4 to take precedence, got %v", conn["host"])
}

func TestConnectionJSON_NilExtra(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type: "ssh",
			Host: "1.2.3.4",
			User: "admin",
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "ssh", conn["type"], "expected type ssh, got %v", conn["type"])
	require.Equal(t, "1.2.3.4", conn["host"], "expected host 1.2.3.4, got %v", conn["host"])
	require.Equal(t, "admin", conn["user"], "expected user admin, got %v", conn["user"])
}

func TestConnectionJSON_EmptyExtra(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
		Type: "aws_instance",
		Name: "web",
		Connection: &config.Connection{
			Type:  "ssh",
			Host:  "1.2.3.4",
			Extra: map[string]any{},
		},
		Attributes: map[string]any{"ami": "ami-123"},
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "ssh", conn["type"], "expected type ssh, got %v", conn["type"])
	require.Equal(t, "1.2.3.4", conn["host"], "expected host 1.2.3.4, got %v", conn["host"])
}

func TestConnectionJSON_ExtraOnlyUnknownFields(t *testing.T) {
	stack := config.NewStack("test")
	require.NoError(t, stack.AddResource(&config.Resource{
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
	}))

	data, err := ToJSON(stack)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	res := mustMap(t, mustMap(t, mustMap(t, obj, "resource"), "aws_instance"), "web")
	conn := mustMap(t, res, "connection")
	require.Equal(t, "ssh", conn["type"], "expected type ssh, got %v", conn["type"])
	require.Equal(t, "1.2.3.4", conn["host"], "expected host 1.2.3.4, got %v", conn["host"])
	require.Equal(t, "deploy", conn["user"], "expected user deploy, got %v", conn["user"])
	require.Equal(t, "10.0.0.1", conn["bastion_host"], "expected bastion_host 10.0.0.1, got %v", conn["bastion_host"])
	require.Equal(t, "bastion", conn["bastion_user"], "expected bastion_user bastion, got %v", conn["bastion_user"])
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
		require.NoError(t, err, "Marshal")
		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m), "Unmarshal")

		require.Equal(t, "aws.west", m["provider"], "provider = %v, want aws.west", m["provider"])
		deps, ok := m["depends_on"].([]any)
		require.True(t, ok, "depends_on: expected []any, got %T", m["depends_on"])
		require.Len(t, deps, 1)
		require.Equal(t, "aws_vpc.main", deps[0], "depends_on = %v, want [aws_vpc.main]", deps)
		require.Equal(t, float64(3), m["count"], "count = %v, want 3", m["count"])
		fe, ok := m["for_each"].(map[string]any)
		require.True(t, ok, "for_each: expected map[string]any, got %T", m["for_each"])
		require.Equal(t, "1", fe["a"], "for_each = %v, want map[a:1]", fe)
		require.Equal(t, "value", m["existing"], "existing = %v, want value", m["existing"])
	})

	t.Run("no meta fields set", func(t *testing.T) {
		rc := resourceConfigJSON{
			Config: map[string]any{"existing": "value"},
		}

		data, err := json.Marshal(rc)
		require.NoError(t, err, "Marshal")
		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m), "Unmarshal")

		for _, key := range []string{"provider", "depends_on", "count", "for_each", "lifecycle", "timeouts", "connection", "provisioner"} {
			_, ok := m[key]
			require.False(t, ok, "%s should not be present", key)
		}
		require.Equal(t, "value", m["existing"], "existing = %v, want value", m["existing"])
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
	require.NoError(t, err, "Marshal")
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m), "Unmarshal")

	inner, ok := m["local-exec"].(map[string]any)
	require.True(t, ok, "expected local-exec key, got keys: %v", keys(m))
	require.Equal(t, "echo hello", inner["command"], "command = %v, want echo hello", inner["command"])
	require.Equal(t, "create", inner["when"], "when = %v, want create", inner["when"])
	require.Equal(t, "continue", inner["on_failure"], "on_failure = %v, want continue", inner["on_failure"])
}

func TestProviderConfigJSON_MarshalJSON(t *testing.T) {
	t.Run("with alias", func(t *testing.T) {
		p := providerConfigJSON{
			Config: map[string]any{"region": "us-east-1"},
			Alias:  "east",
		}
		data, err := json.Marshal(p)
		require.NoError(t, err, "Marshal")
		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m), "Unmarshal")
		require.Equal(t, "us-east-1", m["region"], "region = %v, want us-east-1", m["region"])
		require.Equal(t, "east", m["alias"], "alias = %v, want east", m["alias"])
	})

	t.Run("without alias", func(t *testing.T) {
		p := providerConfigJSON{
			Config: map[string]any{"region": "us-west-2"},
		}
		data, err := json.Marshal(p)
		require.NoError(t, err, "Marshal")
		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m), "Unmarshal")
		_, ok := m["alias"]
		require.False(t, ok, "alias should not be present")
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

	data, err := ToJSON(stack)
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

	data, err := ToJSON(stack)
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

	if _, err := ToJSON(stack); err == nil {
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

	if _, err := ToJSON(stack); err == nil {
		t.Fatal("ToJSON should error even when Attributes is an empty (but non-nil) map")
	}
}
