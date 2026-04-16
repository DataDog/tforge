//go:build integration

package tforge_test

import (
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// requireCLI fails the test immediately if the given CLI binary is not found in PATH.
func requireCLI(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Fatalf("%s not found in PATH", name)
	}
}

// requireDocker fails the test immediately if Docker is not available.
func requireDocker(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		t.Fatalf("Docker not available: %v\n%s", err, out)
	}
}

// tforgeRoot returns the absolute path to the repository root.
func tforgeRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	return root
}

// copyDir recursively copies src to dst, skipping entries whose base names
// match any of the exclude strings.
func copyDir(t *testing.T, src, dst string, excludes ...string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Check if any path component matches an exclude.
		for _, ex := range excludes {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, p := range parts {
				if p == ex {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		t.Fatalf("copying %s to %s: %v", src, dst, err)
	}
}

// normalizeTFJSON unmarshals JSON and strips CDKTF metadata for comparison:
// removes "//" keys, "backend" from terraform block, and false-valued fields.
func normalizeTFJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	cleanJSON(m)
	delete(m, "//")
	if tf, ok := m["terraform"].(map[string]any); ok {
		delete(tf, "backend")
		if len(tf) == 0 {
			delete(m, "terraform")
		}
	}
	return m
}

// cleanJSON recursively removes "//" keys and false-valued entries.
func cleanJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, v := range val {
			if k == "//" {
				delete(val, k)
				continue
			}
			if b, ok := v.(bool); ok && !b {
				delete(val, k)
				continue
			}
			val[k] = cleanJSON(v)
		}
		return val
	case []any:
		for i, item := range val {
			val[i] = cleanJSON(item)
		}
		return val
	default:
		return v
	}
}

// jsonEqual compares two maps by marshaling to sorted JSON. On mismatch,
// it logs both outputs and returns false.
func jsonEqual(t *testing.T, label string, want, got map[string]any) bool {
	t.Helper()
	wantBytes, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	gotBytes, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if string(wantBytes) == string(gotBytes) {
		return true
	}
	t.Errorf("%s: output mismatch\n--- want ---\n%s\n--- got ---\n%s", label, wantBytes, gotBytes)
	return false
}

// runCmd executes a command and returns its combined output. The command
// inherits the current environment plus any extra env vars provided.
func runCmd(t *testing.T, dir string, env []string, name string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.CombinedOutput()
}

// discoverDirs returns subdirectory names under the given path.
func discoverDirs(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}
