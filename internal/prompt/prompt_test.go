package prompt

import (
	"strings"
	"testing"

	"github.com/juuso/auto-command/internal/envctx"
)

func TestBuild(t *testing.T) {
	env := envctx.Context{OS: "linux", Arch: "amd64", Shell: "zsh"}
	msgs := Build(env, `find files named "a b.txt"`, 5)

	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("roles = %q, %q", msgs[0].Role, msgs[1].Role)
	}

	// The raw query is passed through verbatim.
	if msgs[1].Content != `find files named "a b.txt"` {
		t.Errorf("user content = %q", msgs[1].Content)
	}

	sys := msgs[0].Content
	for _, want := range []string{"linux", "amd64", "zsh", `{"suggestions":`, "1 and 5"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt missing %q:\n%s", want, sys)
		}
	}

	// It must not leak richer environment detail.
	for _, forbidden := range []string{"cwd", "directory", "history", "environment variable"} {
		if strings.Contains(strings.ToLower(sys), forbidden) {
			t.Errorf("system prompt should not mention %q", forbidden)
		}
	}
}
