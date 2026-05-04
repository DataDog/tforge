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
// The version is derived from the provider binary filename inside the zip
// (the convention `terraform-provider-<name>_v<version>` from the official
// terraform-provider release tooling).
func FetchSchemaFromBinary(ctx context.Context, source, archivePath string) (*Schema, error) {
	src, err := ParseSource(source)
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

	version, err := unzipProvider(archivePath, overrideDir, src)
	if err != nil {
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

// unzipProvider extracts archivePath into dest and returns the version detected
// from the provider binary filename (`terraform-provider-<name>_v<version>`).
func unzipProvider(archivePath, dest string, src Source) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer r.Close()

	binaryPrefix := "terraform-provider-" + src.Name + "_v"
	var version string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if err := extractZipEntry(f, filepath.Join(dest, name)); err != nil {
			return "", err
		}
		if strings.HasPrefix(name, binaryPrefix) {
			version = strings.TrimPrefix(name, binaryPrefix)
		}
	}
	if version == "" {
		return "", fmt.Errorf("no provider binary matching %s* found in %s", binaryPrefix, archivePath)
	}
	return version, nil
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

