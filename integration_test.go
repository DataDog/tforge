//go:build integration

package tforge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestMigration — end-to-end migration tests (replaces migration-test.sh)
// ---------------------------------------------------------------------------

func TestMigration(t *testing.T) {
	root := tforgeRoot(t)
	migrationDir := filepath.Join(root, "testdata", "migration")

	// Happy-path tests: run every example with each CLI.
	clis := []string{"terraform", "tofu"}
	for _, cli := range clis {
		cli := cli
		t.Run(cli, func(t *testing.T) {
			requireCLI(t, cli)
			requireCLI(t, "tforge")

			for _, name := range discoverDirs(t, migrationDir) {
				name := name
				t.Run(name, func(t *testing.T) {
					testMigrationExample(t, root, migrationDir, name, cli)
				})
			}
		})
	}

	// Failure-mode tests (CLI-independent).
	t.Run("invalid_cdktf_json", func(t *testing.T) {
		requireCLI(t, "tforge")
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte("not json"), 0644); err != nil {
			t.Fatal(err)
		}
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		if err == nil {
			t.Error("expected tforge migrate to fail on invalid cdktf.json")
		}
		t.Logf("output: %s", out)
	})

	t.Run("missing_cdktf_json", func(t *testing.T) {
		requireCLI(t, "tforge")
		dir := t.TempDir()
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		if err == nil {
			t.Error("expected tforge migrate to fail when cdktf.json is missing")
		}
		t.Logf("output: %s", out)
	})

	t.Run("no_providers", func(t *testing.T) {
		requireCLI(t, "tforge")
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(`{"terraformProviders":[]}`), 0644); err != nil {
			t.Fatal(err)
		}
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		if err == nil {
			t.Error("expected tforge migrate to fail with no providers")
		}
		t.Logf("output: %s", out)
	})
}

func testMigrationExample(t *testing.T, root, migrationDir, name, cli string) {
	t.Helper()

	exampleDir := filepath.Join(migrationDir, name)
	cdktfDir := filepath.Join(exampleDir, "cdktf")
	tforgeDir := filepath.Join(exampleDir, "tforge")

	// Check prerequisites.
	if _, err := os.Stat(filepath.Join(cdktfDir, "cdktf.json")); err != nil {
		t.Skipf("missing cdktf/cdktf.json")
	}
	if _, err := os.Stat(filepath.Join(tforgeDir, "main.go")); err != nil {
		t.Skipf("missing tforge/main.go")
	}

	// Find expected output.
	expectedFile := findCdkTFJSON(t, cdktfDir)
	if expectedFile == "" {
		t.Skip("no cdktf.out/.../cdk.tf.json found")
	}

	// Step 1: Copy cdktf/ to temp dir (excluding cdktf.out/).
	tmpDir := t.TempDir()
	copyDir(t, cdktfDir, tmpDir, "cdktf.out")

	// Step 2: Run tforge migrate.
	env := []string{"TFORGE_CLI=" + cli}
	out, err := runCmd(t, tmpDir, env, "tforge", "migrate", "-dir", tmpDir)
	if err != nil {
		t.Fatalf("tforge migrate failed: %v\n%s", err, out)
	}

	// Step 3: Verify cdktf.json deleted.
	if _, err := os.Stat(filepath.Join(tmpDir, "cdktf.json")); !os.IsNotExist(err) {
		t.Error("cdktf.json should have been deleted by tforge migrate")
	}

	// Step 4: Verify at least one generate.go created.
	genFiles := findFiles(t, tmpDir, "generate.go")
	if len(genFiles) == 0 {
		t.Fatal("no generate.go files created by tforge migrate")
	}

	// Step 5: Swap in tforge main.go and go.mod.
	copyFile(t, filepath.Join(tforgeDir, "main.go"), filepath.Join(tmpDir, "main.go"))
	goModSrc, err := os.ReadFile(filepath.Join(tforgeDir, "go.mod"))
	if err != nil {
		t.Fatalf("reading tforge/go.mod: %v", err)
	}
	goMod := string(goModSrc) + "\nreplace github.com/DataDog/tforge => " + root + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Step 6: go generate.
	out, err = runCmd(t, tmpDir, env, "go", "generate", "./...")
	if err != nil {
		t.Fatalf("go generate failed: %v\n%s", err, out)
	}

	// Step 7: go run . → produces main.tf.json in the temp dir.
	out, err = runCmd(t, tmpDir, env, "go", "run", ".")
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, out)
	}

	actual, err := os.ReadFile(filepath.Join(tmpDir, "main.tf.json"))
	if err != nil {
		t.Fatalf("reading main.tf.json: %v", err)
	}

	// Step 8: Normalize and compare.
	expectedData, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("reading expected: %v", err)
	}
	wantNorm := normalizeTFJSON(t, expectedData)
	gotNorm := normalizeTFJSON(t, actual)
	jsonEqual(t, name, wantNorm, gotNorm)
}

// findCdkTFJSON finds the first cdk.tf.json in the cdktf.out directory tree.
func findCdkTFJSON(t *testing.T, cdktfDir string) string {
	t.Helper()
	var found string
	filepath.WalkDir(filepath.Join(cdktfDir, "cdktf.out"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == "cdk.tf.json" && found == "" {
			found = path
		}
		return nil
	})
	return found
}

// findFiles returns all files with the given name under dir.
func findFiles(t *testing.T, dir, name string) []string {
	t.Helper()
	var files []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == name {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// copyFile copies a single file from src to dst.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("writing %s: %v", dst, err)
	}
}

// ---------------------------------------------------------------------------
// TestSnapshots — golden file snapshot tests (replaces Makefile snapshot-tests)
// ---------------------------------------------------------------------------

func TestSnapshots(t *testing.T) {
	root := tforgeRoot(t)
	goldenBase := filepath.Join(root, "testdata", "golden")

	clis := []string{"terraform", "tofu"}
	for _, cli := range clis {
		cli := cli
		t.Run(cli, func(t *testing.T) {
			requireCLI(t, cli)
			requireCLI(t, "tforge")

			// Run go generate with TFORGE_CLI.
			env := []string{"TFORGE_CLI=" + cli}
			out, err := runCmd(t, root, env, "go", "generate", "./testdata/golden/...")
			if err != nil {
				t.Fatalf("go generate failed with %s: %v\n%s", cli, err, out)
			}

			// Compare generated Go code against *.go.golden snapshots.
			t.Run("generated_code", func(t *testing.T) {
				compareGeneratedCodeGoldens(t, root, goldenBase)
			})

			// Compare JSON output against *.golden.json snapshots.
			t.Run("json_output", func(t *testing.T) {
				compareJSONOutputGoldens(t, root, goldenBase)
			})
		})
	}
}

// compareJSONOutputGoldens runs each golden directory's main.go and compares
// the JSON output against main.tf.golden.json.
func compareJSONOutputGoldens(t *testing.T, root, goldenBase string) {
	t.Helper()
	for _, name := range discoverDirs(t, goldenBase) {
		name := name
		t.Run(name, func(t *testing.T) {
			goldenFile := filepath.Join(goldenBase, name, "main.tf.golden.json")
			if _, err := os.Stat(goldenFile); err != nil {
				t.Skipf("no golden file: %v", err)
			}

			// go run ./<dir> → stdout.
			cmd := exec.Command("go", "run", "./testdata/golden/"+name)
			cmd.Dir = root
			actual, err := cmd.Output()
			if err != nil {
				var stderr []byte
				if ee, ok := err.(*exec.ExitError); ok {
					stderr = ee.Stderr
				}
				t.Fatalf("go run failed: %v\n%s", err, stderr)
			}

			if *update {
				if err := os.WriteFile(goldenFile, actual, 0644); err != nil {
					t.Fatalf("updating golden file: %v", err)
				}
				return
			}

			golden, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}

			if !bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(golden)) {
				t.Errorf("output mismatch for %s\n--- golden ---\n%s\n--- actual ---\n%s",
					name, golden, actual)
			}
		})
	}
}

// generatedCodeGoldenExcludes lists provider directories excluded from generated
// code golden comparison. name_collision/aws is excluded because its
// resources_gen.go is ~174 MB, making it impractical to commit as a golden file.
var generatedCodeGoldenExcludes = map[string]bool{
	filepath.Join("name_collision", "aws"): true,
}

// compareGeneratedCodeGoldens finds every provider directory (containing a
// generate.go) under goldenBase, requires that committed *.go.golden files
// exist for each generated file, and compares them byte-for-byte.
// Directories listed in generatedCodeGoldenExcludes are skipped.
func compareGeneratedCodeGoldens(t *testing.T, root, goldenBase string) {
	t.Helper()

	// Discover all provider directories that contain a generate.go.
	var providerDirs []string
	filepath.WalkDir(goldenBase, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "generate.go" {
			providerDirs = append(providerDirs, filepath.Dir(path))
		}
		return nil
	})

	if len(providerDirs) == 0 {
		t.Fatal("no generate.go files found under testdata/golden")
	}

	for _, dir := range providerDirs {
		rel, _ := filepath.Rel(goldenBase, dir)
		if generatedCodeGoldenExcludes[rel] {
			continue
		}

		t.Run(rel, func(t *testing.T) {
			// Find all *_gen.go files produced by go generate.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("reading directory %s: %v", rel, err)
			}

			var genFiles []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), "_gen.go") {
					genFiles = append(genFiles, e.Name())
				}
			}
			if len(genFiles) == 0 {
				t.Fatalf("no *_gen.go files found in %s — did go generate run?", rel)
			}

			for _, name := range genFiles {
				name := name
				t.Run(name, func(t *testing.T) {
					genFile := filepath.Join(dir, name)
					goldenFile := genFile + ".golden"

					if *update {
						src, err := os.ReadFile(genFile)
						if err != nil {
							t.Fatalf("reading generated file: %v", err)
						}
						if err := os.WriteFile(goldenFile, src, 0644); err != nil {
							t.Fatalf("updating golden file: %v", err)
						}
						return
					}

					if _, err := os.Stat(goldenFile); err != nil {
						t.Fatalf("missing golden file %s.golden — run 'make update-snapshots' to create it", name)
					}

					golden, err := os.ReadFile(goldenFile)
					if err != nil {
						t.Fatalf("reading golden file: %v", err)
					}
					actual, err := os.ReadFile(genFile)
					if err != nil {
						t.Fatalf("reading generated file: %v", err)
					}

					if !bytes.Equal(golden, actual) {
						goldenLines := strings.Split(string(golden), "\n")
						actualLines := strings.Split(string(actual), "\n")
						for i := 0; i < len(goldenLines) && i < len(actualLines); i++ {
							if goldenLines[i] != actualLines[i] {
								t.Errorf("generated code mismatch at line %d\n--- golden ---\n%s\n--- actual ---\n%s",
									i+1, goldenLines[i], actualLines[i])
								break
							}
						}
						if len(goldenLines) != len(actualLines) {
							t.Errorf("line count differs: golden=%d actual=%d", len(goldenLines), len(actualLines))
						}
						if !t.Failed() {
							t.Errorf("files differ (binary comparison)")
						}
					}
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestTerraformValidate — validate golden files against multiple TF/OpenTofu
// versions via Docker (replaces validate-golden.sh)
// ---------------------------------------------------------------------------

// toolInfo describes a Terraform/OpenTofu CLI for Docker validation.
type toolInfo struct {
	key   string // "tf" or "tofu"
	image string // Docker image base, e.g. "hashicorp/terraform"
	cmd   string // CLI command name, "terraform" or "tofu"
}

// providerGroup holds golden files that share the same required_providers.
type providerGroup struct {
	hash       string // short hash of canonical required_providers JSON
	configJSON []byte // terraform { required_providers { ... } } as JSON
	files      []string
}

func TestTerraformValidate(t *testing.T) {
	requireDocker(t)

	root := tforgeRoot(t)
	goldenBase := filepath.Join(root, "testdata", "golden")

	// Discover golden files.
	var goldenFiles []string
	for _, name := range discoverDirs(t, goldenBase) {
		f := filepath.Join(goldenBase, name, "main.tf.golden.json")
		if _, err := os.Stat(f); err == nil {
			goldenFiles = append(goldenFiles, f)
		}
	}
	if len(goldenFiles) == 0 {
		t.Fatal("no golden files found")
	}

	// Group by required_providers hash.
	groups := groupByProviders(t, goldenFiles)
	t.Logf("Found %d golden files in %d provider group(s)", len(goldenFiles), len(groups))

	// Parse versions from env. The Makefile is the single source of truth for
	// default versions; run via "make validate-golden" to get them automatically.
	tfVersions := parseVersions(os.Getenv("TERRAFORM_VERSIONS"), "")
	otVersions := parseVersions(os.Getenv("TOFU_VERSIONS"), "")
	if len(tfVersions) == 0 && len(otVersions) == 0 {
		t.Skip("TERRAFORM_VERSIONS and TOFU_VERSIONS not set; run via 'make validate-golden'")
	}

	tools := []toolInfo{
		{"tf", "hashicorp/terraform", "terraform"},
		{"tofu", "ghcr.io/opentofu/opentofu", "tofu"},
	}

	// Ensure init caches.
	ensureInitCaches(t, groups, tools)

	// Validate each version in parallel.
	for _, tool := range tools {
		versions := tfVersions
		if tool.key == "tofu" {
			versions = otVersions
		}
		for _, ver := range versions {
			ver := ver
			tool := tool
			label := tool.cmd + "/" + ver
			image := tool.image + ":" + ver
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				validateVersion(t, image, tool.cmd, tool.key, groups, goldenFiles)
			})
		}
	}
}

func parseVersions(env, defaultVal string) []string {
	s := env
	if s == "" {
		s = defaultVal
	}
	return strings.Fields(s)
}

func groupByProviders(t *testing.T, files []string) map[string]*providerGroup {
	t.Helper()
	groups := make(map[string]*providerGroup)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}

		// Extract required_providers.
		rp := map[string]any{}
		if tf, ok := doc["terraform"].(map[string]any); ok {
			if p, ok := tf["required_providers"].(map[string]any); ok {
				rp = p
			}
		}
		canonical, _ := json.Marshal(rp)
		h := fmt.Sprintf("%x", sha256.Sum256(canonical))[:12]

		if g, ok := groups[h]; ok {
			g.files = append(g.files, f)
		} else {
			// Build a minimal config for init.
			config := map[string]any{
				"terraform": map[string]any{
					"required_providers": rp,
				},
			}
			configJSON, _ := json.Marshal(config)
			groups[h] = &providerGroup{
				hash:       h,
				configJSON: configJSON,
				files:      []string{f},
			}
		}
	}
	return groups
}

func cacheVolName(toolKey, groupHash string) string {
	return fmt.Sprintf("tforge-init-%s-%s", toolKey, groupHash)
}

// ensureInitCaches creates Docker volume init caches for each provider group + tool.
func ensureInitCaches(t *testing.T, groups map[string]*providerGroup, tools []toolInfo) {
	t.Helper()

	pluginCacheDir := "/tmp/tforge-cache"
	os.MkdirAll(pluginCacheDir, 0755)

	for _, group := range groups {
		for _, tool := range tools {
			vol := cacheVolName(tool.key, group.hash)
			initImage := tool.image + ":latest"

			// Create volume if needed.
			exec.Command("docker", "volume", "create", vol).Run()

			// Check if cache is valid by reading config.hash from volume.
			configHash := fmt.Sprintf("%x", sha256.Sum256(group.configJSON))
			checkOut, _ := exec.Command("docker", "run", "--rm",
				"-v", vol+":/cache:ro",
				"alpine", "cat", "/cache/config.hash").Output()

			if strings.TrimSpace(string(checkOut)) == configHash {
				continue // Cache is valid.
			}

			t.Logf("Initializing %s provider cache for group %s...", tool.cmd, group.hash)

			// Reset volume.
			exec.Command("docker", "volume", "rm", vol).Run()
			exec.Command("docker", "volume", "create", vol).Run()

			// Write config to temp file for docker cp.
			tmpConfig, err := os.CreateTemp("", "tforge-config-*.tf.json")
			if err != nil {
				t.Fatalf("creating temp config: %v", err)
			}
			tmpConfig.Write(group.configJSON)
			tmpConfig.Close()
			defer os.Remove(tmpConfig.Name())

			// Create container, copy config, run init.
			script := fmt.Sprintf(
				"mkdir /work && cd /work && cp /tmp/config.tf.json main.tf.json && %s init -backend=false && "+
					"cp -a .terraform/providers /cache/providers && cp .terraform.lock.hcl /cache/lock.hcl && "+
					"echo '%s' > /cache/config.hash",
				tool.cmd, configHash)

			createOut, err := exec.Command("docker", "create",
				"--entrypoint", "sh",
				"-v", vol+":/cache",
				"-v", pluginCacheDir+":/plugin-cache",
				"-e", "TF_PLUGIN_CACHE_DIR=/plugin-cache",
				initImage,
				"-c", script).Output()
			if err != nil {
				t.Fatalf("docker create failed for %s init: %v", tool.cmd, err)
			}
			containerID := strings.TrimSpace(string(createOut))

			// Copy config file into container.
			if out, err := exec.Command("docker", "cp", tmpConfig.Name(), containerID+":/tmp/config.tf.json").CombinedOutput(); err != nil {
				exec.Command("docker", "rm", containerID).Run()
				t.Fatalf("docker cp failed: %v\n%s", err, out)
			}

			// Start container.
			out, err := exec.Command("docker", "start", "-a", containerID).CombinedOutput()
			exec.Command("docker", "rm", containerID).Run()
			if err != nil {
				t.Fatalf("init failed for %s group %s: %v\n%s", tool.cmd, group.hash, err, out)
			}
		}
	}
}

// validateVersion runs terraform/tofu validate in a Docker container for all golden files.
func validateVersion(t *testing.T, image, cmd, toolKey string, groups map[string]*providerGroup, goldenFiles []string) {
	t.Helper()

	pluginCacheDir := "/tmp/tforge-cache"

	// Map each file to its group hash.
	fileGroup := map[string]string{}
	for _, g := range groups {
		for _, f := range g.files {
			fileGroup[f] = g.hash
		}
	}

	// Build volume mount args.
	volArgs := []string{}
	mountedGroups := map[string]bool{}
	for _, f := range goldenFiles {
		gh := fileGroup[f]
		if mountedGroups[gh] {
			continue
		}
		mountedGroups[gh] = true
		vol := cacheVolName(toolKey, gh)
		volArgs = append(volArgs, "-v", vol+":/init-cache-"+gh+":ro")
	}

	// Build in-container script.
	var script strings.Builder
	script.WriteString("set -e\nmkdir -p /work && cd /work\n")

	prevGroup := ""
	for i, f := range goldenFiles {
		gh := fileGroup[f]
		if gh != prevGroup {
			script.WriteString("rm -f /work/.terraform/providers /work/.terraform.lock.hcl\n")
			script.WriteString("mkdir -p /work/.terraform\n")
			fmt.Fprintf(&script, "ln -s /init-cache-%s/providers /work/.terraform/providers\n", gh)
			fmt.Fprintf(&script, "cp /init-cache-%s/lock.hcl /work/.terraform.lock.hcl\n", gh)
			prevGroup = gh
		}
		fmt.Fprintf(&script, "echo '--- %s ---'\n", filepath.Base(filepath.Dir(f)))
		fmt.Fprintf(&script, "cp /tmp/golden_%d.tf.json main.tf.json\n", i)
		fmt.Fprintf(&script, "%s validate\n", cmd)
	}

	// Create container.
	args := []string{"create", "--entrypoint", "sh"}
	args = append(args, volArgs...)
	args = append(args,
		"-v", pluginCacheDir+":/plugin-cache:ro",
		"-e", "TF_PLUGIN_CACHE_DIR=/plugin-cache",
		image,
		"-c", script.String())

	createOut, err := exec.Command("docker", args...).Output()
	if err != nil {
		t.Fatalf("docker create failed: %v", err)
	}
	containerID := strings.TrimSpace(string(createOut))
	defer exec.Command("docker", "rm", containerID).Run()

	// Copy golden files.
	for i, f := range goldenFiles {
		if out, err := exec.Command("docker", "cp", f, fmt.Sprintf("%s:/tmp/golden_%d.tf.json", containerID, i)).CombinedOutput(); err != nil {
			t.Fatalf("docker cp %s failed: %v\n%s", f, err, out)
		}
	}

	// Run.
	out, err := exec.Command("docker", "start", "-a", containerID).CombinedOutput()
	if err != nil {
		t.Errorf("validation failed:\n%s", out)
	} else {
		t.Logf("validation passed:\n%s", out)
	}
}
