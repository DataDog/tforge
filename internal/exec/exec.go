// Package exec provides utilities for executing provider binaries.
//
// This is an internal package not intended for public use.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Runner executes external commands.
type Runner struct {
	// Timeout for command execution (default: 30s)
	Timeout time.Duration

	// Env contains additional environment variables.
	Env []string

	// Dir is the working directory.
	Dir string
}

// NewRunner creates a new command runner with default settings.
func NewRunner() *Runner {
	return &Runner{
		Timeout: 30 * time.Second,
	}
}

// Result contains the result of a command execution.
type Result struct {
	// Stdout contains standard output.
	Stdout []byte

	// Stderr contains standard error.
	Stderr []byte

	// ExitCode is the process exit code.
	ExitCode int

	// Duration is how long the command took.
	Duration time.Duration
}

// Run executes a command and returns the result.
func (r *Runner) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if r.Dir != "" {
		cmd.Dir = r.Dir
	}

	// Combine current environment with additional vars
	cmd.Env = append(os.Environ(), r.Env...)

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("running command: %w", err)
		}
	}

	return result, nil
}

// RunSimple executes a command and returns stdout, or an error if it fails.
func (r *Runner) RunSimple(ctx context.Context, name string, args ...string) ([]byte, error) {
	result, err := r.Run(ctx, name, args...)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("command failed (exit %d): %s", result.ExitCode, result.Stderr)
	}
	return result.Stdout, nil
}

// Which finds the path to an executable.
func Which(name string) (string, error) {
	return exec.LookPath(name)
}

// Exists checks if an executable exists in PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
