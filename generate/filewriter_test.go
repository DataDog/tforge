package generate

import "testing"

func TestToGoName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"monitor", "Monitor"},
		{"datadog_monitor", "DatadogMonitor"},
		{"api_key", "APIKey"},
		{"instance_id", "InstanceID"},
		{"ec2_instance", "EC2Instance"},
		{"foo_bar_baz", "FooBarBaz"},
		{"", ""},
		{"a", "A"},
		{"http_url", "HTTPURL"},
		{"vpc_id", "VPCID"},
		// Edge cases from issue #27
		{"_internal", "Internal"},
		{"__", ""},
		{"3d_model", "X3dModel"},
		{"http_api_url_id", "HTTPAPIURLID"},
		{"a_b_c_d_e", "ABCDE"},
		{"__leading__underscores__", "LeadingUnderscores"},
		{"123", "X123"},
		{"0_count", "X0Count"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toGoName(tt.input)
			if got != tt.want {
				t.Errorf("toGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
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
			if got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCommonAbbreviation(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ID", true},
		{"API", true},
		{"URL", true},
		{"VPC", true},
		{"FOO", false},
		{"id", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isCommonAbbreviation(tt.input)
			if got != tt.want {
				t.Errorf("isCommonAbbreviation(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
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
			if got != tt.want {
				t.Errorf("cleanDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
