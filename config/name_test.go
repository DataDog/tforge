package config

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain alphanumeric passthrough", "my_resource_1", "my_resource_1"},
		{"hyphen preserved", "mcnulty-apm-py3", "mcnulty-apm-py3"},
		{"slash becomes double-dash", "api/v2", "api--v2"},
		{"curly braces and colon stripped",
			"api/v2/services/definitions/{path:path}_GET",
			"api--v2--services--definitions--pathpath_GET",
		},
		{"realistic monitor name",
			"mcnulty-apm-py3_sustained_5xx_api/v2/services/definitions/{path:path}_GET",
			"mcnulty-apm-py3_sustained_5xx_api--v2--services--definitions--pathpath_GET",
		},
		{"gate suffix preserved",
			"mcnulty-apm-py3_sustained_5xx_api/v2/services/definitions/{path:path}_GET_gate",
			"mcnulty-apm-py3_sustained_5xx_api--v2--services--definitions--pathpath_GET_gate",
		},
		{"dots stripped", "foo.bar.baz", "foobarbaz"},
		{"spaces stripped", "hello world", "helloworld"},
		{"unicode stripped", "café", "caf"},
		{"idempotent on already-sanitized input",
			"api--v2--pathpath_GET",
			"api--v2--pathpath_GET",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeName(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// Idempotence: a second pass must be a no-op.
			if got2 := SanitizeName(got); got2 != got {
				t.Errorf("SanitizeName is not idempotent: SanitizeName(%q) = %q", got, got2)
			}
		})
	}
}

func TestAddResource_SanitizesName(t *testing.T) {
	s := NewStack("test")
	r := &Resource{
		Type: "datadog_monitor",
		Name: "mcnulty_api/v2/{path:path}_GET",
	}
	err := s.AddResource(r)
	if err != nil {
		t.Fatalf("AddResource: unexpected error: %v", err)
	}
	// Raw Name is preserved on the underlying Resource.
	if r.Name != "mcnulty_api/v2/{path:path}_GET" {
		t.Errorf("caller r.Name = %q, want it unchanged", r.Name)
	}
	// SanitizedName() exposes the Terraform-valid form.
	want := "mcnulty_api--v2--pathpath_GET"
	if got := s.Resources[0].AsResource().SanitizedName(); got != want {
		t.Errorf("stored SanitizedName() = %q, want %q", got, want)
	}
}

func TestAddResource_RejectsNameWithOnlyInvalidChars(t *testing.T) {
	s := NewStack("test")
	err := s.AddResource(&Resource{Type: "datadog_monitor", Name: "{}"})
	if err == nil {
		t.Fatal("expected error for name that sanitizes to empty")
	}
}

func TestAddDataSource_SanitizesName(t *testing.T) {
	s := NewStack("test")
	d := &DataSource{
		Type: "datadog_synthetics_test",
		Name: "team/web/{id}",
	}
	err := s.AddDataSource(d)
	if err != nil {
		t.Fatalf("AddDataSource: unexpected error: %v", err)
	}
	if d.Name != "team/web/{id}" {
		t.Errorf("caller d.Name = %q, want it unchanged", d.Name)
	}
	want := "team--web--id"
	if got := s.DataSources[0].AsDataSource().SanitizedName(); got != want {
		t.Errorf("stored SanitizedName() = %q, want %q", got, want)
	}
}

func TestAddModule_SanitizesName(t *testing.T) {
	s := NewStack("test")
	err := s.AddModule(Module{
		Name:   "platform/networking/{region}",
		Source: "./modules/net",
	})
	if err != nil {
		t.Fatalf("AddModule: unexpected error: %v", err)
	}
	want := "platform--networking--region"
	if got := s.Modules[0].SanitizedName(); got != want {
		t.Errorf("stored SanitizedName() = %q, want %q", got, want)
	}
}
