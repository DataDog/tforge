package config

import (
	"testing"
)

func TestNewStack(t *testing.T) {
	s := NewStack("my-stack")

	if s.Name != "my-stack" {
		t.Fatalf("Name = %q, want %q", s.Name, "my-stack")
	}
	if s.Backend != nil {
		t.Fatal("Backend should be nil")
	}
	// Slices and maps start nil; verify they are usable via the Add*/Set* methods.
	if len(s.RequiredProviders) != 0 {
		t.Fatal("RequiredProviders should be empty")
	}
	if len(s.Providers) != 0 {
		t.Fatal("Providers should be empty")
	}
	if len(s.Resources) != 0 {
		t.Fatal("Resources should be empty")
	}
	if len(s.DataSources) != 0 {
		t.Fatal("DataSources should be empty")
	}
	if len(s.Modules) != 0 {
		t.Fatal("Modules should be empty")
	}
	if len(s.Variables) != 0 {
		t.Fatal("Variables should be empty")
	}
	if len(s.Outputs) != 0 {
		t.Fatal("Outputs should be empty")
	}
	if len(s.Locals) != 0 {
		t.Fatal("Locals should be empty")
	}
}

func TestSetBackend(t *testing.T) {
	s := NewStack("test")
	b := Backend{Type: "s3", Config: map[string]any{"bucket": "my-bucket"}}
	s.SetBackend(b)

	if s.Backend == nil {
		t.Fatal("Backend should not be nil after SetBackend")
	}
	if s.Backend.Type != "s3" {
		t.Fatalf("Backend.Type = %q, want %q", s.Backend.Type, "s3")
	}
}

func TestAddProvider(t *testing.T) {
	t.Run("without version and source", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws"})

		if len(s.Providers) != 1 {
			t.Fatalf("len(Providers) = %d, want 1", len(s.Providers))
		}
		if s.Providers[0].Name != "aws" {
			t.Fatalf("Providers[0].Name = %q, want %q", s.Providers[0].Name, "aws")
		}
		if len(s.RequiredProviders) != 0 {
			t.Fatalf("len(RequiredProviders) = %d, want 0", len(s.RequiredProviders))
		}
	})

	t.Run("with version and source", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Version: "~> 5.0", Source: "hashicorp/aws"})

		if len(s.Providers) != 1 {
			t.Fatalf("len(Providers) = %d, want 1", len(s.Providers))
		}
		rp, ok := s.RequiredProviders["aws"]
		if !ok {
			t.Fatal("RequiredProviders should contain 'aws'")
		}
		if rp.Source != "hashicorp/aws" {
			t.Fatalf("RequiredProviders[aws].Source = %q, want %q", rp.Source, "hashicorp/aws")
		}
		if rp.Version != "~> 5.0" {
			t.Fatalf("RequiredProviders[aws].Version = %q, want %q", rp.Version, "~> 5.0")
		}
	})

	t.Run("with version only", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Version: "~> 5.0"})

		if len(s.RequiredProviders) != 0 {
			t.Fatal("RequiredProviders should be empty when Source is missing")
		}
	})

	t.Run("with source only", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Source: "hashicorp/aws"})

		if len(s.RequiredProviders) != 0 {
			t.Fatal("RequiredProviders should be empty when Version is missing")
		}
	})
}

func TestAddResource(t *testing.T) {
	tests := []struct {
		name    string
		res     Resource
		wantErr bool
	}{
		{
			name:    "valid resource",
			res:     Resource{Type: "aws_instance", Name: "web"},
			wantErr: false,
		},
		{
			name:    "empty type",
			res:     Resource{Type: "", Name: "web"},
			wantErr: true,
		},
		{
			name:    "empty name",
			res:     Resource{Type: "aws_instance", Name: ""},
			wantErr: true,
		},
		{
			name:    "both empty",
			res:     Resource{Type: "", Name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStack("test")
			err := s.AddResource(&tt.res)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(s.Resources) != 1 {
				t.Fatalf("len(Resources) = %d, want 1", len(s.Resources))
			}
			if tt.wantErr && len(s.Resources) != 0 {
				t.Fatalf("len(Resources) = %d, want 0 on error", len(s.Resources))
			}
		})
	}
}

func TestAddDataSource(t *testing.T) {
	tests := []struct {
		name    string
		ds      DataSource
		wantErr bool
	}{
		{
			name:    "valid data source",
			ds:      DataSource{Type: "aws_ami", Name: "latest"},
			wantErr: false,
		},
		{
			name:    "empty type",
			ds:      DataSource{Type: "", Name: "latest"},
			wantErr: true,
		},
		{
			name:    "empty name",
			ds:      DataSource{Type: "aws_ami", Name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStack("test")
			err := s.AddDataSource(&tt.ds)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddDataSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(s.DataSources) != 1 {
				t.Fatalf("len(DataSources) = %d, want 1", len(s.DataSources))
			}
			if tt.wantErr && len(s.DataSources) != 0 {
				t.Fatalf("len(DataSources) = %d, want 0 on error", len(s.DataSources))
			}
		})
	}
}

func TestAddResourceDuplicate(t *testing.T) {
	s := NewStack("test")
	err := s.AddResource(&Resource{Type: "aws_instance", Name: "web", Attributes: map[string]any{"ami": "first"}})
	if err != nil {
		t.Fatalf("first AddResource() error = %v", err)
	}

	err = s.AddResource(&Resource{Type: "aws_instance", Name: "web", Attributes: map[string]any{"ami": "second"}})
	if err == nil {
		t.Fatal("second AddResource() with same type and name should return error")
	}
	if len(s.Resources) != 1 {
		t.Fatalf("len(Resources) = %d, want 1 after duplicate rejected", len(s.Resources))
	}

	// Different name should succeed
	err = s.AddResource(&Resource{Type: "aws_instance", Name: "api"})
	if err != nil {
		t.Fatalf("AddResource() with different name error = %v", err)
	}

	// Different type should succeed
	err = s.AddResource(&Resource{Type: "aws_s3_bucket", Name: "web"})
	if err != nil {
		t.Fatalf("AddResource() with different type error = %v", err)
	}

	if len(s.Resources) != 3 {
		t.Fatalf("len(Resources) = %d, want 3", len(s.Resources))
	}
}

func TestAddResource_MutationPropagates(t *testing.T) {
	s := NewStack("test")
	r := &Resource{Type: "aws_instance", Name: "web"}
	if err := s.AddResource(r); err != nil {
		t.Fatalf("AddResource error = %v", err)
	}
	r.Attributes = map[string]any{"ami": "ami-123"}

	if s.Resources[0].AsResource().Attributes["ami"] != "ami-123" {
		t.Fatalf("post-add mutation not visible: got %v", s.Resources[0].AsResource().Attributes)
	}
}

func TestAddResource_NilRejected(t *testing.T) {
	s := NewStack("test")
	if err := s.AddResource(nil); err == nil {
		t.Fatal("AddResource(nil) should return an error")
	}
	if err := s.AddResource((*Resource)(nil)); err == nil {
		t.Fatal("AddResource((*Resource)(nil)) should return an error")
	}
}

func TestAddDataSourceDuplicate(t *testing.T) {
	s := NewStack("test")
	err := s.AddDataSource(&DataSource{Type: "aws_ami", Name: "latest", Attributes: map[string]any{"filter": "first"}})
	if err != nil {
		t.Fatalf("first AddDataSource() error = %v", err)
	}

	err = s.AddDataSource(&DataSource{Type: "aws_ami", Name: "latest", Attributes: map[string]any{"filter": "second"}})
	if err == nil {
		t.Fatal("second AddDataSource() with same type and name should return error")
	}
	if len(s.DataSources) != 1 {
		t.Fatalf("len(DataSources) = %d, want 1 after duplicate rejected", len(s.DataSources))
	}

	// Different name should succeed
	err = s.AddDataSource(&DataSource{Type: "aws_ami", Name: "base"})
	if err != nil {
		t.Fatalf("AddDataSource() with different name error = %v", err)
	}

	if len(s.DataSources) != 2 {
		t.Fatalf("len(DataSources) = %d, want 2", len(s.DataSources))
	}
}

func TestAddModule(t *testing.T) {
	tests := []struct {
		name    string
		mod     Module
		wantErr bool
	}{
		{
			name:    "valid module",
			mod:     Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"},
			wantErr: false,
		},
		{
			name:    "empty name",
			mod:     Module{Name: "", Source: "terraform-aws-modules/eks/aws"},
			wantErr: true,
		},
		{
			name:    "empty source",
			mod:     Module{Name: "eks", Source: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStack("test")
			err := s.AddModule(tt.mod)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddModule() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(s.Modules) != 1 {
				t.Fatalf("len(Modules) = %d, want 1", len(s.Modules))
			}
			if tt.wantErr && len(s.Modules) != 0 {
				t.Fatalf("len(Modules) = %d, want 0 on error", len(s.Modules))
			}
		})
	}
}

func TestAddModuleDuplicate(t *testing.T) {
	s := NewStack("test")
	err := s.AddModule(Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"})
	if err != nil {
		t.Fatalf("first AddModule() error = %v", err)
	}

	err = s.AddModule(Module{Name: "eks", Source: "other/module"})
	if err == nil {
		t.Fatal("second AddModule() with same name should return error")
	}
	if len(s.Modules) != 1 {
		t.Fatalf("len(Modules) = %d, want 1 after duplicate rejected", len(s.Modules))
	}

	// Different name should succeed
	err = s.AddModule(Module{Name: "vpc", Source: "terraform-aws-modules/vpc/aws"})
	if err != nil {
		t.Fatalf("AddModule() with different name error = %v", err)
	}
	if len(s.Modules) != 2 {
		t.Fatalf("len(Modules) = %d, want 2", len(s.Modules))
	}
}

// TestAddModule_PreservesCallerName verifies the no-mutation contract:
// AddModule must not rewrite m.Name. The sanitized form is reachable only
// through Module.SanitizedName(). This closes the codex feedback at PR #15.
func TestAddModule_PreservesCallerName(t *testing.T) {
	s := NewStack("test")
	const raw = "1-bad/leading"
	m := Module{Name: raw, Source: "terraform-aws-modules/eks/aws"}
	if err := s.AddModule(m); err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	// The caller-visible value must not be touched.
	if m.Name != raw {
		t.Errorf("caller m.Name = %q, want %q (unchanged)", m.Name, raw)
	}
	// The stored copy also keeps the raw Name; only SanitizedName() applies the rule.
	stored := s.Modules[0]
	if stored.Name != raw {
		t.Errorf("stored Name = %q, want %q", stored.Name, raw)
	}
	if got, want := stored.SanitizedName(), SanitizeName(raw); got != want {
		t.Errorf("stored SanitizedName() = %q, want %q", got, want)
	}
}

func TestModuleAddress(t *testing.T) {
	m := Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"}
	if got := m.Address(); got != "module.eks" {
		t.Errorf("Address() = %q, want %q", got, "module.eks")
	}
}

func TestModuleRef(t *testing.T) {
	m := Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"}
	if got := m.Ref("cluster_endpoint"); got != "${module.eks.cluster_endpoint}" {
		t.Errorf("Ref() = %q, want %q", got, "${module.eks.cluster_endpoint}")
	}
}

func TestNewStackIncludesModules(t *testing.T) {
	s := NewStack("test")
	if len(s.Modules) != 0 {
		t.Fatal("Modules should be empty")
	}
	// Verify modules can be added (lazy initialization works)
	if err := s.AddModule(Module{Name: "test", Source: "./mod"}); err != nil {
		t.Fatalf("AddModule on fresh stack: %v", err)
	}
	if len(s.Modules) != 1 {
		t.Fatalf("len(Modules) = %d, want 1", len(s.Modules))
	}
}

func TestAddVariable(t *testing.T) {
	s := NewStack("test")
	if err := s.AddVariable(Variable{Name: "region", Type: "string"}); err != nil {
		t.Fatalf("AddVariable(region) error = %v", err)
	}
	if err := s.AddVariable(Variable{Name: "count", Type: "number"}); err != nil {
		t.Fatalf("AddVariable(count) error = %v", err)
	}

	if len(s.Variables) != 2 {
		t.Fatalf("len(Variables) = %d, want 2", len(s.Variables))
	}
	if s.Variables[0].Name != "region" {
		t.Fatalf("Variables[0].Name = %q, want %q", s.Variables[0].Name, "region")
	}
	if s.Variables[1].Name != "count" {
		t.Fatalf("Variables[1].Name = %q, want %q", s.Variables[1].Name, "count")
	}
}

func TestAddVariableErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		s := NewStack("test")
		if err := s.AddVariable(Variable{Name: "", Type: "string"}); err == nil {
			t.Fatal("AddVariable with empty name should return error")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		s := NewStack("test")
		if err := s.AddVariable(Variable{Name: "region", Type: "string"}); err != nil {
			t.Fatalf("first AddVariable() error = %v", err)
		}
		if err := s.AddVariable(Variable{Name: "region", Type: "number"}); err == nil {
			t.Fatal("second AddVariable() with same name should return error")
		}
		if len(s.Variables) != 1 {
			t.Fatalf("len(Variables) = %d, want 1 after duplicate rejected", len(s.Variables))
		}
	})
}

func TestAddOutput(t *testing.T) {
	s := NewStack("test")
	if err := s.AddOutput(Output{Name: "ip", Value: "${aws_instance.web.public_ip}"}); err != nil {
		t.Fatalf("AddOutput() error = %v", err)
	}

	if len(s.Outputs) != 1 {
		t.Fatalf("len(Outputs) = %d, want 1", len(s.Outputs))
	}
	if s.Outputs[0].Name != "ip" {
		t.Fatalf("Outputs[0].Name = %q, want %q", s.Outputs[0].Name, "ip")
	}
}

func TestAddOutputErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		s := NewStack("test")
		if err := s.AddOutput(Output{Name: "", Value: "foo"}); err == nil {
			t.Fatal("AddOutput with empty name should return error")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		s := NewStack("test")
		if err := s.AddOutput(Output{Name: "ip", Value: "1.2.3.4"}); err != nil {
			t.Fatalf("first AddOutput() error = %v", err)
		}
		if err := s.AddOutput(Output{Name: "ip", Value: "5.6.7.8"}); err == nil {
			t.Fatal("second AddOutput() with same name should return error")
		}
		if len(s.Outputs) != 1 {
			t.Fatalf("len(Outputs) = %d, want 1 after duplicate rejected", len(s.Outputs))
		}
	})
}

func TestSetLocal(t *testing.T) {
	s := NewStack("test")
	s.SetLocal("env", "production")
	s.SetLocal("tags", map[string]string{"team": "infra"})

	if s.Locals["env"] != "production" {
		t.Fatalf("Locals[env] = %v, want %q", s.Locals["env"], "production")
	}
	tags, ok := s.Locals["tags"].(map[string]string)
	if !ok {
		t.Fatal("Locals[tags] should be map[string]string")
	}
	if tags["team"] != "infra" {
		t.Fatalf("Locals[tags][team] = %q, want %q", tags["team"], "infra")
	}

	// overwrite
	s.SetLocal("env", "staging")
	if s.Locals["env"] != "staging" {
		t.Fatalf("Locals[env] = %v after overwrite, want %q", s.Locals["env"], "staging")
	}
}
