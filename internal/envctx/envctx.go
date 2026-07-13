// Package envctx gathers the minimal environment context that is sent to the
// model. It deliberately exposes only the operating system, architecture, and
// shell — never the working directory, directory listings, environment
// variables, or shell history.
package envctx

import (
	"os"
	"path/filepath"
	"runtime"
)

// Context is the minimal, non-sensitive environment description shared with the
// model. Every field is either a compile-time constant or derived from a single
// environment variable; nothing here reveals what the user is doing.
type Context struct {
	// OS is the operating system, e.g. "darwin" or "linux".
	OS string
	// Arch is the CPU architecture, e.g. "arm64" or "amd64".
	Arch string
	// Shell is the shell name, e.g. "zsh".
	Shell string
}

// Detect returns the current environment context. The shell is derived from the
// $SHELL variable's basename and falls back to "zsh" when it is unset.
func Detect() Context {
	return Context{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Shell: detectShell(),
	}
}

// detectShell reads $SHELL and returns its basename. Only the shell name is
// used; the full path is never sent to the model.
func detectShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return filepath.Base(s)
	}
	return "zsh"
}
