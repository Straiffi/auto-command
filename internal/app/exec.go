package app

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

// CommandRunner executes a chosen shell command. It is an interface so Run can
// be exercised with a fake in tests; shellRunner is the real implementation.
type CommandRunner interface {
	// Run executes command and returns its exit code. A non-nil error means the
	// command could not be started at all; a command that ran but exited
	// non-zero returns that code with a nil error.
	Run(ctx context.Context, command string) (exitCode int, err error)
}

// shellRunner runs a command through the user's shell, inheriting the given
// standard streams so output reaches the terminal and interactive commands
// work. It resolves $SHELL, falling back to /bin/sh.
type shellRunner struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (r shellRunner) Run(ctx context.Context, command string) (int, error) {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, sh, "-c", command)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// The command ran but exited non-zero; surface its code, not an error.
			return ee.ExitCode(), nil
		}
		// The command could not be started (shell missing, etc.).
		return 1, err
	}
	return 0, nil
}
