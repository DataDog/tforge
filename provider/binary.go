package provider

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FetchSchemaFromBinary fetches a provider schema from a local Terraform provider
// release zip (e.g., terraform-provider-datadog_3.39.0_linux_amd64.zip from
// DataDog's GitHub releases) using OpenTofu/Terraform's dev_overrides mechanism.
// No network access: the registry is never consulted.
//
// The version is derived from the archive filename, which must follow the
// Terraform provider registry protocol convention
// `terraform-provider-<name>_<version>_<os>_<arch>.zip`.
func FetchSchemaFromBinary(ctx context.Context, source, archivePath string) (*Schema, error) {
	src, err := ParseSource(source)
	if err != nil {
		return nil, err
	}

	version, err := parseVersionFromArchive(archivePath, src)
	if err != nil {
		return nil, err
	}

	cliPath, err := findCLI()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "tforge-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	overrideDir := filepath.Join(tmpDir, "providers")
	if err := os.MkdirAll(overrideDir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating overrides dir: %w", err)
	}

	if err := unzipProvider(archivePath, overrideDir, src); err != nil {
		return nil, fmt.Errorf("extracting provider archive: %w", err)
	}

	cliConfig := fmt.Sprintf(`provider_installation {
  dev_overrides {
    %q = %q
  }
  direct {}
}
`, src.String(), overrideDir)
	cliConfigPath := filepath.Join(tmpDir, "terraformrc")
	if err := os.WriteFile(cliConfigPath, []byte(cliConfig), filePerm); err != nil {
		return nil, fmt.Errorf("writing CLI config: %w", err)
	}

	if err := writeTerraformConfig(tmpDir, src, version); err != nil {
		return nil, err
	}

	cmdEnv := filterEnv(os.Environ(), "OTEL_TRACES_EXPORTER")
	cmdEnv = append(cmdEnv,
		"TF_CLI_CONFIG_FILE="+cliConfigPath,
		"CHECKPOINT_DISABLE=1",
	)

	// dev_overrides bypasses init; go straight to providers schema.
	return runProvidersSchema(ctx, cliPath, tmpDir, cmdEnv, src, version)
}

// parseVersionFromArchive extracts the provider version from a release zip's
// filename. Official Terraform provider release archives are named
// `terraform-provider-<name>_<version>_<os>_<arch>.zip` per the registry
// protocol: https://developer.hashicorp.com/terraform/internals/provider-registry-protocol#find-a-provider-package
func parseVersionFromArchive(archivePath string, src Source) (string, error) {
	base := filepath.Base(archivePath)
	expected := fmt.Sprintf("terraform-provider-%s_<version>_<os>_<arch>.zip", src.Name)
	prefix := "terraform-provider-" + src.Name + "_"
	if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, ".zip") {
		return "", fmt.Errorf("archive name %q does not match expected format %s", base, expected)
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".zip")
	parts := strings.Split(rest, "_")
	if len(parts) != 3 || parts[0] == "" {
		return "", fmt.Errorf("archive name %q does not match expected format %s", base, expected)
	}
	return parts[0], nil
}

// unzipProvider extracts archivePath into dest and verifies the archive
// contains a provider plugin binary (`terraform-provider-<name>*`).
func unzipProvider(archivePath, dest string, src Source) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer r.Close()

	binaryPrefix := "terraform-provider-" + src.Name
	var foundBinary bool
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if err := extractZipEntry(f, filepath.Join(dest, name)); err != nil {
			return err
		}
		if strings.HasPrefix(name, binaryPrefix) {
			foundBinary = true
		}
	}
	if !foundBinary {
		return fmt.Errorf("no provider binary matching %s* found in %s", binaryPrefix, archivePath)
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening %s in zip: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}
