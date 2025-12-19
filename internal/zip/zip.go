// Package zip provides ZIP archive extraction utilities.
//
// This is an internal package not intended for public use.
package zip

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract extracts a ZIP archive to a destination directory.
func Extract(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}

	for _, f := range r.File {
		if err := extractFile(f, dst); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}

	return nil
}

// extractFile extracts a single file from a ZIP archive.
func extractFile(f *zip.File, dst string) error {
	// Sanitize the file path to prevent zip slip attacks
	path, err := sanitizePath(dst, f.Name)
	if err != nil {
		return err
	}

	// Handle directories
	if f.FileInfo().IsDir() {
		return os.MkdirAll(path, f.Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Open source file in archive
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create destination file
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy content
	_, err = io.Copy(out, rc)
	return err
}

// sanitizePath ensures the extracted file path is within the destination directory.
// This prevents "zip slip" attacks where malicious archives contain paths like "../../../etc/passwd".
func sanitizePath(dst, name string) (string, error) {
	// Clean the name to remove any ".." components
	name = filepath.Clean(name)

	// Join with destination
	path := filepath.Join(dst, name)

	// Ensure the result is within dst
	if !strings.HasPrefix(path, filepath.Clean(dst)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path: %s", name)
	}

	return path, nil
}

// List returns the names of all files in a ZIP archive.
func List(src string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	names := make([]string, len(r.File))
	for i, f := range r.File {
		names[i] = f.Name
	}

	return names, nil
}

// Contains checks if a ZIP archive contains a file with the given name.
func Contains(src, name string) (bool, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == name {
			return true, nil
		}
	}

	return false, nil
}

