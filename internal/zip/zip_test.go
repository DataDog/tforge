package zip

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// createTestZip creates a ZIP file with the given file entries (name -> content).
func createTestZip(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("creating entry %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("writing entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	return path
}

func TestExtract_BasicFiles(t *testing.T) {
	srcDir := t.TempDir()
	zipPath := createTestZip(t, srcDir, map[string]string{
		"hello.txt":         "hello world",
		"subdir/nested.txt": "nested content",
	})

	dstDir := t.TempDir()
	if err := Extract(zipPath, dstDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check hello.txt
	data, err := os.ReadFile(filepath.Join(dstDir, "hello.txt"))
	if err != nil {
		t.Fatalf("reading hello.txt: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", data)
	}

	// Check nested file
	data, err = os.ReadFile(filepath.Join(dstDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("reading subdir/nested.txt: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("expected 'nested content', got %q", data)
	}
}

func TestExtract_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(badPath, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Extract(badPath, dir); err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestExtract_ZipSlipPrevention(t *testing.T) {
	// Create a zip with a path traversal entry using raw zip header
	srcDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating file: %v", err)
	}

	w := zip.NewWriter(f)
	// Create a file with a "../" prefix to attempt zip slip
	header := &zip.FileHeader{Name: "../../../tmp/evil.txt"}
	header.Method = zip.Deflate
	fw, err := w.CreateHeader(header)
	if err != nil {
		f.Close()
		t.Fatalf("creating header: %v", err)
	}
	if _, err = fw.Write([]byte("evil")); err != nil {
		t.Fatal(err)
	}
	w.Close()
	f.Close()

	dstDir := t.TempDir()
	err = Extract(zipPath, dstDir)
	if err == nil {
		t.Fatal("expected error for zip slip path traversal")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	zipPath := createTestZip(t, dir, map[string]string{
		"a.txt": "aaa",
		"b.txt": "bbb",
		"c.txt": "ccc",
	})

	names, err := List(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(names)
	expected := []string{"a.txt", "b.txt", "c.txt"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d files, got %d", len(expected), len(names))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], name)
		}
	}
}

func TestList_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(badPath, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := List(badPath)
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestContains_Existing(t *testing.T) {
	dir := t.TempDir()
	zipPath := createTestZip(t, dir, map[string]string{
		"found.txt": "content",
		"other.txt": "other",
	})

	found, err := Contains(zipPath, "found.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected Contains to return true for existing file")
	}
}

func TestContains_Missing(t *testing.T) {
	dir := t.TempDir()
	zipPath := createTestZip(t, dir, map[string]string{
		"found.txt": "content",
	})

	found, err := Contains(zipPath, "missing.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected Contains to return false for missing file")
	}
}

func TestContains_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.zip")
	if err := os.WriteFile(badPath, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Contains(badPath, "anything")
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}
