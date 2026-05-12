package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewStack(t *testing.T) {
	s := NewStack("my-stack")

	require.Equal(t, "my-stack", s.Name)
	require.Nil(t, s.Backend, "Backend should be nil")
	// Slices and maps start nil; verify they are usable via the Add*/Set* methods.
	require.Empty(t, s.RequiredProviders, "RequiredProviders should be empty")
	require.Empty(t, s.Providers, "Providers should be empty")
	require.Empty(t, s.Resources, "Resources should be empty")
	require.Empty(t, s.DataSources, "DataSources should be empty")
	require.Empty(t, s.Modules, "Modules should be empty")
	require.Empty(t, s.Variables, "Variables should be empty")
	require.Empty(t, s.Outputs, "Outputs should be empty")
	require.Empty(t, s.Locals, "Locals should be empty")
}

func TestSetBackend(t *testing.T) {
	s := NewStack("test")
	b := Backend{Type: "s3", Config: map[string]any{"bucket": "my-bucket"}}
	s.SetBackend(b)

	require.NotNil(t, s.Backend, "Backend should not be nil after SetBackend")
	require.Equal(t, "s3", s.Backend.Type)
}

func TestAddProvider(t *testing.T) {
	t.Run("without version and source", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws"})

		require.Len(t, s.Providers, 1)
		require.Equal(t, "aws", s.Providers[0].Name)
		require.Empty(t, s.RequiredProviders)
	})

	t.Run("with version and source", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Version: "~> 5.0", Source: "hashicorp/aws"})

		require.Len(t, s.Providers, 1)
		rp, ok := s.RequiredProviders["aws"]
		require.True(t, ok, "RequiredProviders should contain 'aws'")
		require.Equal(t, "hashicorp/aws", rp.Source)
		require.Equal(t, "~> 5.0", rp.Version)
	})

	t.Run("with version only", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Version: "~> 5.0"})

		require.Empty(t, s.RequiredProviders, "RequiredProviders should be empty when Source is missing")
	})

	t.Run("with source only", func(t *testing.T) {
		s := NewStack("test")
		s.AddProvider(Provider{Name: "aws", Source: "hashicorp/aws"})

		require.Empty(t, s.RequiredProviders, "RequiredProviders should be empty when Version is missing")
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
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, s.Resources, "len(Resources) should be 0 on error")
			} else {
				require.NoError(t, err)
				require.Len(t, s.Resources, 1)
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
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, s.DataSources, "len(DataSources) should be 0 on error")
			} else {
				require.NoError(t, err)
				require.Len(t, s.DataSources, 1)
			}
		})
	}
}

func TestAddResourceDuplicate(t *testing.T) {
	s := NewStack("test")
	err := s.AddResource(&Resource{Type: "aws_instance", Name: "web", Attributes: map[string]any{"ami": "first"}})
	require.NoError(t, err, "first AddResource() error = %v", err)

	err = s.AddResource(&Resource{Type: "aws_instance", Name: "web", Attributes: map[string]any{"ami": "second"}})
	require.Error(t, err, "second AddResource() with same type and name should return error")
	require.Len(t, s.Resources, 1, "len(Resources) should be 1 after duplicate rejected")

	// Different name should succeed
	err = s.AddResource(&Resource{Type: "aws_instance", Name: "api"})
	require.NoError(t, err, "AddResource() with different name error = %v", err)

	// Different type should succeed
	err = s.AddResource(&Resource{Type: "aws_s3_bucket", Name: "web"})
	require.NoError(t, err, "AddResource() with different type error = %v", err)

	require.Len(t, s.Resources, 3)
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
	require.NoError(t, err, "first AddDataSource() error = %v", err)

	err = s.AddDataSource(&DataSource{Type: "aws_ami", Name: "latest", Attributes: map[string]any{"filter": "second"}})
	require.Error(t, err, "second AddDataSource() with same type and name should return error")
	require.Len(t, s.DataSources, 1, "len(DataSources) should be 1 after duplicate rejected")

	// Different name should succeed
	err = s.AddDataSource(&DataSource{Type: "aws_ami", Name: "base"})
	require.NoError(t, err, "AddDataSource() with different name error = %v", err)

	require.Len(t, s.DataSources, 2)
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
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, s.Modules, "len(Modules) should be 0 on error")
			} else {
				require.NoError(t, err)
				require.Len(t, s.Modules, 1)
			}
		})
	}
}

func TestAddModuleDuplicate(t *testing.T) {
	s := NewStack("test")
	err := s.AddModule(Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"})
	require.NoError(t, err, "first AddModule() error = %v", err)

	err = s.AddModule(Module{Name: "eks", Source: "other/module"})
	require.Error(t, err, "second AddModule() with same name should return error")
	require.Len(t, s.Modules, 1, "len(Modules) should be 1 after duplicate rejected")

	// Different name should succeed
	err = s.AddModule(Module{Name: "vpc", Source: "terraform-aws-modules/vpc/aws"})
	require.NoError(t, err, "AddModule() with different name error = %v", err)
	require.Len(t, s.Modules, 2)
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
	require.Equal(t, "module.eks", m.Address())
}

func TestModuleRef(t *testing.T) {
	m := Module{Name: "eks", Source: "terraform-aws-modules/eks/aws"}
	require.Equal(t, "${module.eks.cluster_endpoint}", m.Ref("cluster_endpoint"))
}

func TestNewStackIncludesModules(t *testing.T) {
	s := NewStack("test")
	require.Empty(t, s.Modules, "Modules should be empty")
	// Verify modules can be added (lazy initialization works)
	require.NoError(t, s.AddModule(Module{Name: "test", Source: "./mod"}), "AddModule on fresh stack")
	require.Len(t, s.Modules, 1)
}

func TestAddVariable(t *testing.T) {
	s := NewStack("test")
	require.NoError(t, s.AddVariable(Variable{Name: "region", Type: "string"}), "AddVariable(region)")
	require.NoError(t, s.AddVariable(Variable{Name: "count", Type: "number"}), "AddVariable(count)")

	require.Len(t, s.Variables, 2)
	require.Equal(t, "region", s.Variables[0].Name)
	require.Equal(t, "count", s.Variables[1].Name)
}

func TestAddVariableErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		s := NewStack("test")
		require.Error(t, s.AddVariable(Variable{Name: "", Type: "string"}), "AddVariable with empty name should return error")
	})

	t.Run("duplicate", func(t *testing.T) {
		s := NewStack("test")
		require.NoError(t, s.AddVariable(Variable{Name: "region", Type: "string"}), "first AddVariable()")
		require.Error(t, s.AddVariable(Variable{Name: "region", Type: "number"}), "second AddVariable() with same name should return error")
		require.Len(t, s.Variables, 1, "len(Variables) should be 1 after duplicate rejected")
	})
}

func TestAddOutput(t *testing.T) {
	s := NewStack("test")
	require.NoError(t, s.AddOutput(Output{Name: "ip", Value: "${aws_instance.web.public_ip}"}))

	require.Len(t, s.Outputs, 1)
	require.Equal(t, "ip", s.Outputs[0].Name)
}

func TestAddOutputErrors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		s := NewStack("test")
		require.Error(t, s.AddOutput(Output{Name: "", Value: "foo"}), "AddOutput with empty name should return error")
	})

	t.Run("duplicate", func(t *testing.T) {
		s := NewStack("test")
		require.NoError(t, s.AddOutput(Output{Name: "ip", Value: "1.2.3.4"}), "first AddOutput()")
		require.Error(t, s.AddOutput(Output{Name: "ip", Value: "5.6.7.8"}), "second AddOutput() with same name should return error")
		require.Len(t, s.Outputs, 1, "len(Outputs) should be 1 after duplicate rejected")
	})
}

func TestSetLocal(t *testing.T) {
	s := NewStack("test")
	s.SetLocal("env", "production")
	s.SetLocal("tags", map[string]string{"team": "infra"})

	require.Equal(t, "production", s.Locals["env"])
	tags, ok := s.Locals["tags"].(map[string]string)
	require.True(t, ok, "Locals[tags] should be map[string]string")
	require.Equal(t, "infra", tags["team"])

	// overwrite
	s.SetLocal("env", "staging")
	require.Equal(t, "staging", s.Locals["env"])
}
