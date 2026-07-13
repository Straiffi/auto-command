package shell

import (
	"strings"
	"testing"
)

// TestZshWidgetInvariants locks in the ticket's hard constraints on the zsh
// snippet: it must place the command in the buffer for review and must never
// submit or run it automatically.
func TestZshWidgetInvariants(t *testing.T) {
	if strings.TrimSpace(Zsh) == "" {
		t.Fatal("embedded zsh widget is empty")
	}

	mustContain := []string{
		"zle -N",  // registers the widget
		"bindkey", // binds it to a key
		"BUFFER",  // reads the query / sets the buffer
		"^G",      // documented default key
		"reset-prompt",
	}
	for _, s := range mustContain {
		if !strings.Contains(Zsh, s) {
			t.Errorf("embedded zsh widget missing %q", s)
		}
	}

	// The command must never be executed for the user.
	if strings.Contains(Zsh, "accept-line") {
		t.Error("embedded zsh widget calls accept-line; it must never auto-execute")
	}
}
