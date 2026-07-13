package envctx

import (
	"runtime"
	"testing"
)

func TestDetect_ShellFromEnv(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/zsh")
	got := Detect()
	if got.Shell != "zsh" {
		t.Errorf("Shell = %q, want zsh", got.Shell)
	}
	if got.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", got.OS, runtime.GOOS)
	}
	if got.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", got.Arch, runtime.GOARCH)
	}
}

func TestDetect_ShellFallback(t *testing.T) {
	t.Setenv("SHELL", "")
	if got := Detect().Shell; got != "zsh" {
		t.Errorf("Shell = %q, want zsh fallback", got)
	}
}
