package provider

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersionFromArchive(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	tests := []struct {
		name    string
		archive string
		want    string
		wantErr bool
	}{
		{"canonical", "terraform-provider-datadog_3.39.0_linux_amd64.zip", "3.39.0", false},
		{"v-prefix version", "terraform-provider-datadog_v3.39.0_darwin_arm64.zip", "v3.39.0", false},
		{"absolute path", "/tmp/releases/terraform-provider-datadog_3.39.0_linux_amd64.zip", "3.39.0", false},
		{"wrong name prefix", "terraform-provider-aws_3.39.0_linux_amd64.zip", "", true},
		{"non-zip suffix", "terraform-provider-datadog_3.39.0_linux_amd64.tar.gz", "", true},
		{"only one segment", "terraform-provider-datadog_3.39.0.zip", "", true},
		{"two segments", "terraform-provider-datadog_3.39.0_linux.zip", "", true},
		{"four segments", "terraform-provider-datadog_3.39.0_linux_amd64_extra.zip", "", true},
		{"empty version", "terraform-provider-datadog__linux_amd64.zip", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionFromArchive(tt.archive, src)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseVersionFromArchive(%q) = %q, want error", tt.archive, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersionFromArchive(%q) returned error: %v", tt.archive, err)
			}
			if got != tt.want {
				t.Errorf("parseVersionFromArchive(%q) = %q, want %q", tt.archive, got, tt.want)
			}
		})
	}
}

func TestUnzipProvider_ExtractsBinary(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	binaryName := "terraform-provider-datadog_v3.39.0_linux_amd64"
	archivePath := buildTestZip(t, map[string][]byte{
		binaryName: []byte("#!/fakebinary\n"),
	})
	dest := t.TempDir()

	if err := unzipProvider(archivePath, dest, src); err != nil {
		t.Fatalf("unzipProvider returned error: %v", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("ReadDir(%q) returned error: %v", dest, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in dest, got %d", len(entries))
	}
	if entries[0].Name() != binaryName {
		t.Errorf("extracted file = %q, want %q", entries[0].Name(), binaryName)
	}
}

func TestUnzipProvider_MissingBinary(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	archivePath := buildTestZip(t, map[string][]byte{
		"LICENSE": []byte("MIT\n"),
	})

	err := unzipProvider(archivePath, t.TempDir(), src)
	if err == nil {
		t.Fatal("unzipProvider returned nil error, want missing-binary error")
	}
	if !strings.Contains(err.Error(), "no provider binary") {
		t.Errorf("error = %q, want it to mention 'no provider binary'", err.Error())
	}
}

func TestUnzipProvider_NonExistentArchive(t *testing.T) {
	src := Source{Namespace: "datadog", Name: "datadog"}
	err := unzipProvider(filepath.Join(t.TempDir(), "does-not-exist.zip"), t.TempDir(), src)
	if err == nil {
		t.Fatal("unzipProvider returned nil error, want open-failure error")
	}
	if !strings.Contains(err.Error(), "opening") {
		t.Errorf("error = %q, want it to mention 'opening'", err.Error())
	}
}

// buildTestZip writes a zip archive into the test's temp dir containing the
// given files (name -> content) and returns the archive path. Caller does not
// need to clean up; t.TempDir handles it.
func buildTestZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("creating test zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("zip write(%q): %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	return archivePath
}
