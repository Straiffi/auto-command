package app

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestShellRunnerExitCode verifies the real runner reports the executed
// command's exit code (not an error) and streams output to the given writer.
func TestShellRunnerExitCode(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	var out bytes.Buffer
	r := shellRunner{stdin: os.Stdin, stdout: &out, stderr: &out}

	code, err := r.Run(context.Background(), "printf hello; exit 7")
	if err != nil {
		t.Fatalf("Run returned error %v, want nil for a command that ran", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
	if out.String() != "hello" {
		t.Fatalf("captured output = %q, want %q", out.String(), "hello")
	}
}

// TestShellRunnerZeroExit covers the success path.
func TestShellRunnerZeroExit(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	r := shellRunner{stdin: os.Stdin, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	code, err := r.Run(context.Background(), "true")
	if err != nil || code != 0 {
		t.Fatalf("Run(true) = (%d, %v), want (0, nil)", code, err)
	}
}
