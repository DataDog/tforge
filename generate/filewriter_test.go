package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/require"
)

// newTestGenerator returns a Generator with default initialisms for testing.
func newTestGenerator() *Generator {
	return New(Config{})
}

func TestToGoName(t *testing.T) {
	g := newTestGenerator()

	tests := []struct {
		input string
		want  string
	}{
		// Basic conversion
		{"monitor", "Monitor"},
		{"foo_bar_baz", "FooBarBaz"},
		{"", ""},
		{"a", "A"},

		// Adjacent initialisms: each underscore-delimited segment is
		// resolved independently, then concatenated. This follows Go
		// convention (e.g. net/http uses HTTPSQL-style names).
		{"api_http", "APIHTTP"},
		{"http_url", "HTTPURL"},
		{"http_api_url_id", "HTTPAPIURLID"},
		{"vpc_id", "VPCID"},
		{"api_url_ttl", "APIURLTTL"},
		{"iam_arn", "IAMARN"},

		// Common Terraform resource/attribute names
		{"datadog_monitor", "DatadogMonitor"},
		{"api_key", "APIKey"},
		{"instance_id", "InstanceID"},
		{"ec2_instance", "EC2Instance"},
		{"s3_bucket", "S3Bucket"},
		{"iam_role", "IAMRole"},
		{"vpc_security_group", "VPCSecurityGroup"},
		{"rds_cluster", "RDSCluster"},
		{"acl_rule", "ACLRule"},
		{"dns_record", "DNSRecord"},
		{"ssl_certificate", "SSLCertificate"},
		{"ssh_key", "SSHKey"},
		{"api_url", "APIURL"},
		{"notify_no_data", "NotifyNoData"},
		{"no_data_timeframe", "NoDataTimeframe"},
		{"renotify_interval", "RenotifyInterval"},
		{"evaluation_delay", "EvaluationDelay"},
		{"require_full_window", "RequireFullWindow"},
		{"enable_log_collection", "EnableLogCollection"},
		{"force_delete", "ForceDelete"},
		{"tags", "Tags"},
		{"ttl", "TTL"},
		{"ip_address", "IPAddress"},
		{"http_method", "HTTPMethod"},
		{"json_body", "JSONBody"},
		{"sql_query", "SQLQuery"},
		{"cpu_utilization", "CPUUtilization"},
		{"os_type", "OSType"},
		{"vm_size", "VMSize"},

		// Edge cases from issue #27
		{"_internal", "Internal"},
		{"__", ""},
		{"3d_model", "X3dModel"},
		{"a_b_c_d_e", "ABCDE"},
		{"__leading__underscores__", "LeadingUnderscores"},
		{"123", "X123"},
		{"0_count", "X0Count"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := g.toGoName(tt.input)
			require.Equal(t, tt.want, got, "toGoName(%q) = %q, want %q", tt.input, got, tt.want)
		})
	}
}

func TestToGoNameCustomInitialisms(t *testing.T) {
	g := New(Config{Initialisms: []string{"ID", "API"}})

	tests := []struct {
		input string
		want  string
	}{
		{"api_key", "APIKey"},
		{"instance_id", "InstanceID"},
		// VPC is not in custom list, so it gets capitalized normally
		{"vpc_id", "VpcID"},
		// HTTP is not in custom list either
		{"http_url", "HttpUrl"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := g.toGoName(tt.input)
			require.Equal(t, tt.want, got, "toGoName(%q) = %q, want %q", tt.input, got, tt.want)
		})
	}
}

func TestToGoNameEmptyInitialisms(t *testing.T) {
	g := New(Config{Initialisms: []string{}})

	// With empty initialisms, nothing gets special treatment
	got := g.toGoName("api_key")
	require.Equal(t, "ApiKey", got, "toGoName(%q) = %q, want %q", "api_key", got, "ApiKey")
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"a", "A"},
		{"hello", "Hello"},
		{"Hello", "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			require.Equal(t, tt.want, got, "capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
		})
	}
}

func TestDefaultInitialisms(t *testing.T) {
	g := newTestGenerator()

	// Go canonical initialisms should be recognized
	for _, s := range []string{"ID", "API", "URL", "HTTP", "JSON", "SQL"} {
		require.True(t, g.initialisms[s], "expected %q in default initialisms", s)
	}

	// Terraform/cloud terms should be recognized
	for _, s := range []string{"VPC", "ARN", "IAM", "EC2", "S3", "OS"} {
		require.True(t, g.initialisms[s], "expected %q in default initialisms", s)
	}

	// Random strings should not be recognized
	for _, s := range []string{"FOO", "BAR", "CLOUD"} {
		require.False(t, g.initialisms[s], "did not expect %q in default initialisms", s)
	}
}

func TestWriteFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out_gen.go")

	f := jen.NewFile("testpkg")
	f.Func().Id("Hello").Params().String().Block(
		jen.Return(jen.Lit("hello")),
	)

	require.NoError(t, WriteFile(f, path), "WriteFile")

	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading output")
	require.Contains(t, string(data), "func Hello()", "output does not contain expected function")
}

func TestWriteFile_MkdirAllFailure(t *testing.T) {
	// Create a file where a directory is expected, so MkdirAll fails.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0644))

	// Try to write under the file (which is not a directory).
	path := filepath.Join(blocker, "subdir", "out_gen.go")

	f := jen.NewFile("testpkg")
	err := WriteFile(f, path)
	require.Error(t, err, "expected error for MkdirAll failure, got nil")
	require.Contains(t, err.Error(), "creating directory", "expected 'creating directory' in error, got: %v", err)
}

func TestWriteFile_WriteFailure(t *testing.T) {
	// Create a read-only directory so os.WriteFile fails.
	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(readonlyDir, 0755))
	// Make directory read-only after creation.
	require.NoError(t, os.Chmod(readonlyDir, 0555))
	t.Cleanup(func() {
		// Restore permissions so t.TempDir cleanup can remove it.
		os.Chmod(readonlyDir, 0755) //nolint:errcheck
	})

	path := filepath.Join(readonlyDir, "out_gen.go")

	f := jen.NewFile("testpkg")
	f.Func().Id("Hello").Params().String().Block(
		jen.Return(jen.Lit("hello")),
	)

	err := WriteFile(f, path)
	require.Error(t, err, "expected error for write failure, got nil")
	require.Contains(t, err.Error(), "writing file", "expected 'writing file' in error, got: %v", err)
}

func TestCleanDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims and collapses spaces", "  hello  world  ", "hello world"},
		{"collapses newlines", "line1\nline2", "line1 line2"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanDescription(tt.input)
			require.Equal(t, tt.want, got, "cleanDescription(%q) = %q, want %q", tt.input, got, tt.want)
		})
	}
}
