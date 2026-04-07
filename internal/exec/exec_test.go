package exec

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewRunner_DefaultTimeout(t *testing.T) {
	r := NewRunner()
	if r.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", r.Timeout)
	}
}

func TestRun_EchoSuccess(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	result, err := r.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(string(result.Stdout), "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", result.Stdout)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestRun_FalseExitCode(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	result, err := r.Run(ctx, "false")
	if err != nil {
		t.Fatalf("expected no error for non-zero exit, got: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestRunSimple_EchoSuccess(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	out, err := r.RunSimple(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("expected output to contain 'hello', got %q", out)
	}
}

func TestRunSimple_FalseReturnsError(t *testing.T) {
	r := NewRunner()
	ctx := context.Background()

	_, err := r.RunSimple(ctx, "false")
	if err == nil {
		t.Fatal("expected error for failed command")
	}
	if !strings.Contains(err.Error(), "exit") {
		t.Errorf("expected error to mention exit code, got: %v", err)
	}
}

func TestWhich_Go(t *testing.T) {
	path, err := Which("go")
	if err != nil {
		t.Fatalf("expected to find 'go', got error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for 'go'")
	}
}

func TestWhich_Nonexistent(t *testing.T) {
	_, err := Which("nonexistent_binary_xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestExists_Go(t *testing.T) {
	if !Exists("go") {
		t.Error("expected Exists('go') to return true")
	}
}

func TestExists_Nonexistent(t *testing.T) {
	if Exists("nonexistent_binary_xyz") {
		t.Error("expected Exists('nonexistent_binary_xyz') to return false")
	}
}
