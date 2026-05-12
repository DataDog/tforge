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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// pluginCacheDir is the host directory used to cache Terraform/OpenTofu
// provider plugins across Docker validation runs.
const pluginCacheDir = "/tmp/tforge-cache"

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
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte("not json"), 0644))
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		require.Error(t, err, "expected tforge migrate to fail on invalid cdktf.json")
		t.Logf("output: %s", out)
	})

	t.Run("missing_cdktf_json", func(t *testing.T) {
		requireCLI(t, "tforge")
		dir := t.TempDir()
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		require.Error(t, err, "expected tforge migrate to fail when cdktf.json is missing")
		t.Logf("output: %s", out)
	})

	t.Run("no_providers", func(t *testing.T) {
		requireCLI(t, "tforge")
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cdktf.json"), []byte(`{"terraformProviders":[]}`), 0644))
		out, err := runCmd(t, dir, nil, "tforge", "migrate", "-dir", dir)
		require.Error(t, err, "expected tforge migrate to fail with no providers")
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
	require.NoError(t, err, "tforge migrate failed: %v\n%s", err, out)

	// Step 3: Verify cdktf.json deleted.
	_, err = os.Stat(filepath.Join(tmpDir, "cdktf.json"))
	require.True(t, os.IsNotExist(err), "cdktf.json should have been deleted by tforge migrate")

	// Step 4: Verify at least one generate.go created.
	genFiles := findFiles(t, tmpDir, "generate.go")
	require.NotEmpty(t, genFiles, "no generate.go files created by tforge migrate")

	// Step 5: Swap in tforge main.go and go.mod.
	copyFile(t, filepath.Join(tforgeDir, "main.go"), filepath.Join(tmpDir, "main.go"))
	goModSrc, err := os.ReadFile(filepath.Join(tforgeDir, "go.mod"))
	require.NoError(t, err, "reading tforge/go.mod")
	goMod := string(goModSrc) + "\nreplace github.com/DataDog/tforge => " + root + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644), "writing go.mod")

	// Step 6: go generate.
	out, err = runCmd(t, tmpDir, env, "go", "generate", "./...")
	require.NoError(t, err, "go generate failed: %v\n%s", err, out)

	// Step 7: go run . → produces main.tf.json in the temp dir.
	out, err = runCmd(t, tmpDir, env, "go", "run", ".")
	require.NoError(t, err, "go run failed: %v\n%s", err, out)

	actual, err := os.ReadFile(filepath.Join(tmpDir, "main.tf.json"))
	require.NoError(t, err, "reading main.tf.json")

	// Step 8: Normalize and compare.
	expectedData, err := os.ReadFile(expectedFile)
	require.NoError(t, err, "reading expected")
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
	require.NoError(t, err, "reading %s", src)
	require.NoError(t, os.WriteFile(dst, data, 0644), "writing %s", dst)
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

			// Run go generate for each golden directory that isn't excluded.
			// Generating all directories (./testdata/golden/...) would include
			// the AWS provider which takes minutes to generate and compile.
			env := []string{"TFORGE_CLI=" + cli}
			for _, name := range discoverDirs(t, goldenBase) {
				if goldenExcludes[name] {
					continue
				}
				out, err := runCmd(t, root, env, "go", "generate", "./testdata/golden/"+name+"/...")
				require.NoError(t, err, "go generate %s failed with %s: %v\n%s", name, cli, err, out)
			}

			// Compare generated Go code against *.go.golden snapshots.
			t.Run("generated_code", func(t *testing.T) {
				compareGeneratedCodeGoldens(t, goldenBase)
			})

			// Run each golden directory and compare JSON output.
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
			if goldenExcludes[name] {
				t.Skipf("excluded from JSON output comparison (too large)")
				return
			}

			goldenFile := filepath.Join(goldenBase, name, "main.tf.golden.json")
			_, err := os.Stat(goldenFile)
			require.NoError(t, err, "missing golden file %s — run 'make update-snapshots' to create it", goldenFile)

			// go run ./<dir> → stdout.
			cmd := exec.Command("go", "run", "./testdata/golden/"+name)
			cmd.Dir = root
			actual, err := cmd.Output()
			if err != nil {
				var stderr []byte
				if ee, ok := err.(*exec.ExitError); ok {
					stderr = ee.Stderr
				}
				require.NoError(t, err, "go run failed: %v\n%s", err, stderr)
			}

			if *update {
				require.NoError(t, os.WriteFile(goldenFile, actual, 0644), "updating golden file")
				return
			}

			golden, err := os.ReadFile(goldenFile)
			require.NoError(t, err, "reading golden file")

			if !bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(golden)) {
				t.Errorf("output mismatch for %s\n--- golden ---\n%s\n--- actual ---\n%s",
					name, golden, actual)
			}
		})
	}
}

// goldenExcludes lists directories excluded from generated code golden
// comparison and JSON output tests. name_collision is excluded because its AWS
// provider generates ~174 MB of Go code, making it impractical to commit as a
// golden file and too slow to compile in CI.
var goldenExcludes = map[string]bool{
	"name_collision": true,
}

// isGoldenExcluded returns true if the given relative path is excluded or is a
// child of an excluded directory.
func isGoldenExcluded(rel string) bool {
	if goldenExcludes[rel] {
		return true
	}
	for exc := range goldenExcludes {
		if strings.HasPrefix(rel, exc+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// compareGeneratedCodeGoldens verifies every golden test directory is accounted
// for: directories with a generate.go must have committed *.go.golden files
// (unless explicitly excluded), and directories without generate.go are flagged
// as having no generated code. This ensures new golden directories cannot
// silently skip the generated code comparison.
func compareGeneratedCodeGoldens(t *testing.T, goldenBase string) {
	t.Helper()

	// Build a map of provider directories (containing generate.go) keyed by
	// their path relative to goldenBase (e.g. "datadog/datadog").
	providerDirSet := map[string]string{} // rel -> abs
	filepath.WalkDir(goldenBase, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "generate.go" {
			rel, _ := filepath.Rel(goldenBase, filepath.Dir(path))
			providerDirSet[rel] = filepath.Dir(path)
		}
		return nil
	})

	// Verify every top-level golden directory is accounted for.
	for _, name := range discoverDirs(t, goldenBase) {
		hasGen := false
		for rel := range providerDirSet {
			if strings.HasPrefix(rel, name+string(filepath.Separator)) || rel == name {
				hasGen = true
				break
			}
		}
		if !hasGen {
			t.Run(name, func(t *testing.T) {
				t.Skipf("no generate.go — no generated code to compare")
			})
		}
	}

	require.NotEmpty(t, providerDirSet, "no generate.go files found under testdata/golden")

	for rel, dir := range providerDirSet {
		if isGoldenExcluded(rel) {
			t.Run(rel, func(t *testing.T) {
				t.Skipf("excluded from generated code golden comparison (too large)")
			})
			continue
		}

		t.Run(rel, func(t *testing.T) {
			// Find all *_gen.go files produced by go generate and all
			// committed *.go.golden files. Compare both sets to detect
			// regressions where the generator stops emitting a file.
			entries, err := os.ReadDir(dir)
			require.NoError(t, err, "reading directory %s", rel)

			genSet := map[string]bool{}
			goldenSet := map[string]bool{}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.HasSuffix(e.Name(), "_gen.go.golden") {
					// "foo_gen.go.golden" → "foo_gen.go"
					goldenSet[strings.TrimSuffix(e.Name(), ".golden")] = true
				} else if strings.HasSuffix(e.Name(), "_gen.go") {
					genSet[e.Name()] = true
				}
			}

			// Fail if a committed golden has no corresponding generated file.
			for g := range goldenSet {
				if !genSet[g] {
					t.Errorf("golden file %s.golden exists but %s was not generated — generator may have regressed", g, g)
				}
			}

			require.NotEmpty(t, genSet, "no *_gen.go files found in %s — did go generate run?", rel)

			genFiles := make([]string, 0, len(genSet))
			for name := range genSet {
				genFiles = append(genFiles, name)
			}
			sort.Strings(genFiles)

			for _, name := range genFiles {
				name := name
				t.Run(name, func(t *testing.T) {
					genFile := filepath.Join(dir, name)
					goldenFile := genFile + ".golden"

					if *update {
						src, err := os.ReadFile(genFile)
						require.NoError(t, err, "reading generated file")
						require.NoError(t, os.WriteFile(goldenFile, src, 0644), "updating golden file")
						return
					}

					_, err := os.Stat(goldenFile)
					require.NoError(t, err, "missing golden file %s.golden — run 'make update-snapshots' to create it", name)

					golden, err := os.ReadFile(goldenFile)
					require.NoError(t, err, "reading golden file")
					actual, err := os.ReadFile(genFile)
					require.NoError(t, err, "reading generated file")

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
	require.NotEmpty(t, goldenFiles, "no golden files found")

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
		require.NoError(t, err, "reading %s", f)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(data, &doc), "parsing %s", f)

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

	require.NoError(t, os.MkdirAll(pluginCacheDir, 0755), "creating plugin cache dir")

	for _, group := range groups {
		for _, tool := range tools {
			vol := cacheVolName(tool.key, group.hash)
			initImage := tool.image + ":latest"

			// Create volume if needed.
			out, err := exec.Command("docker", "volume", "create", vol).CombinedOutput()
			require.NoError(t, err, "docker volume create %s failed: %v\n%s", vol, err, out)

			// Check if cache is valid by reading config.hash from volume.
			configHash := fmt.Sprintf("%x", sha256.Sum256(group.configJSON))
			checkOut, _ := exec.Command("docker", "run", "--rm",
				"-v", vol+":/cache:ro",
				"alpine", "cat", "/cache/config.hash").Output()

			if strings.TrimSpace(string(checkOut)) == configHash {
				continue // Cache is valid.
			}

			t.Logf("Initializing %s provider cache for group %s...", tool.cmd, group.hash)

			// Reset volume: rm is best-effort (may not exist yet), create must succeed.
			exec.Command("docker", "volume", "rm", vol).Run() //nolint:errcheck // best-effort cleanup
			out, err = exec.Command("docker", "volume", "create", vol).CombinedOutput()
			require.NoError(t, err, "docker volume create %s failed: %v\n%s", vol, err, out)

			// Write config to temp file for docker cp.
			tmpConfig, err := os.CreateTemp("", "tforge-config-*.tf.json")
			require.NoError(t, err, "creating temp config")
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
			require.NoError(t, err, "docker create failed for %s init", tool.cmd)
			containerID := strings.TrimSpace(string(createOut))

			// Copy config file into container.
			out, err = exec.Command("docker", "cp", tmpConfig.Name(), containerID+":/tmp/config.tf.json").CombinedOutput()
			if err != nil {
				exec.Command("docker", "rm", containerID).Run()
				require.NoError(t, err, "docker cp failed: %v\n%s", err, out)
			}

			// Start container.
			out, err = exec.Command("docker", "start", "-a", containerID).CombinedOutput()
			exec.Command("docker", "rm", containerID).Run()
			require.NoError(t, err, "init failed for %s group %s: %v\n%s", tool.cmd, group.hash, err, out)
		}
	}
}

// validateVersion runs terraform/tofu validate in a Docker container for all golden files.
func validateVersion(t *testing.T, image, cmd, toolKey string, groups map[string]*providerGroup, goldenFiles []string) {
	t.Helper()

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
	require.NoError(t, err, "docker create failed")
	containerID := strings.TrimSpace(string(createOut))
	defer exec.Command("docker", "rm", containerID).Run()

	// Copy golden files.
	for i, f := range goldenFiles {
		out, err := exec.Command("docker", "cp", f, fmt.Sprintf("%s:/tmp/golden_%d.tf.json", containerID, i)).CombinedOutput()
		require.NoError(t, err, "docker cp %s failed: %v\n%s", f, err, out)
	}

	// Run.
	out, err := exec.Command("docker", "start", "-a", containerID).CombinedOutput()
	if err != nil {
		t.Errorf("validation failed:\n%s", out)
	} else {
		t.Logf("validation passed:\n%s", out)
	}
}
