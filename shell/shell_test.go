package shell

import (
	"strings"
	"testing"
)

// TestZshWidgetInvariants locks in the ticket's hard constraints on the zsh
// snippet: it must run and print the chosen command (via accept-line), mirroring
// default acmd behavior, while staying registered and bound to Ctrl-G.
func TestZshWidgetInvariants(t *testing.T) {
	if strings.TrimSpace(Zsh) == "" {
		t.Fatal("embedded zsh widget is empty")
	}

	mustContain := []string{
		"zle -N",      // registers the widget
		"bindkey",     // binds it to a key
		"BUFFER",      // reads the query / sets the buffer
		"^G",          // documented default key
		"accept-line", // runs and prints the chosen command
	}
	for _, s := range mustContain {
		if !strings.Contains(Zsh, s) {
			t.Errorf("embedded zsh widget missing %q", s)
		}
	}
}
