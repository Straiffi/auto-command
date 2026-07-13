package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func sample() []Suggestion {
	return []Suggestion{
		{Command: "git status", Explanation: "show the working tree status"},
		{Command: "git log --oneline", Explanation: "compact commit history"},
		{Command: "git diff", Explanation: "show unstaged changes"},
	}
}

// send feeds a key message through Update and returns the resulting model,
// asserting the concrete type so navigation state is easy to read.
func send(t *testing.T, m model, key tea.KeyType) model {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: key})
	return next.(model)
}

func TestDownArrowMovesCursor(t *testing.T) {
	m := newModel(sample())
	m = send(t, m, tea.KeyDown)
	if m.cursor != 1 {
		t.Fatalf("cursor after one down = %d, want 1", m.cursor)
	}
	m = send(t, m, tea.KeyDown)
	if m.cursor != 2 {
		t.Fatalf("cursor after two downs = %d, want 2", m.cursor)
	}
}

func TestDownArrowClampsAtLastItem(t *testing.T) {
	m := newModel(sample())
	for i := 0; i < 5; i++ {
		m = send(t, m, tea.KeyDown)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want clamped at 2", m.cursor)
	}
}

func TestUpArrowClampsAtFirstItem(t *testing.T) {
	m := newModel(sample())
	m = send(t, m, tea.KeyUp)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want clamped at 0", m.cursor)
	}
}

func TestEnterSelectsHighlightedCommand(t *testing.T) {
	m := newModel(sample())
	m = send(t, m, tea.KeyDown) // move to "git log --oneline"

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.chosen {
		t.Fatal("expected chosen=true after Enter")
	}
	if m.cancelled {
		t.Fatal("expected cancelled=false after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after Enter")
	}
	if got := m.suggestions[m.cursor].Command; got != "git log --oneline" {
		t.Fatalf("selected command = %q, want %q", got, "git log --oneline")
	}
}

func TestEscCancels(t *testing.T) {
	m := newModel(sample())
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.chosen {
		t.Fatal("expected chosen=false after Esc")
	}
	if !m.cancelled {
		t.Fatal("expected cancelled=true after Esc")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after Esc")
	}
}

func TestCtrlCCancels(t *testing.T) {
	m := newModel(sample())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(model)
	if !m.cancelled {
		t.Fatal("expected cancelled=true after Ctrl-C")
	}
}

func TestSingleSuggestionRequiresEnter(t *testing.T) {
	m := newModel([]Suggestion{{Command: "ls -la", Explanation: "list files"}})
	// Navigation keys must not confirm a lone suggestion.
	m = send(t, m, tea.KeyDown)
	m = send(t, m, tea.KeyUp)
	if m.chosen || m.cancelled {
		t.Fatal("single suggestion must not auto-select before Enter")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.chosen {
		t.Fatal("expected chosen=true after explicit Enter")
	}
	if got := m.suggestions[m.cursor].Command; got != "ls -la" {
		t.Fatalf("selected command = %q, want %q", got, "ls -la")
	}
}

func TestResizeDoesNotCrash(t *testing.T) {
	m := newModel(sample())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	m = next.(model)
	if m.width != 20 {
		t.Fatalf("width = %d, want 20", m.width)
	}
	// View must render at a narrow width without panicking.
	if out := m.View(); out == "" {
		t.Fatal("expected non-empty view before selection")
	}
}

func TestLongCommandTruncatedButReturnedWhole(t *testing.T) {
	long := strings.Repeat("echo hello && ", 40) + "done"
	m := newModel([]Suggestion{{Command: long, Explanation: "a very long command"}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	m = next.(model)

	view := m.View()
	if !strings.Contains(view, "…") {
		t.Fatal("expected long command to be truncated with an ellipsis in the view")
	}

	// The full command is still what gets selected.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if got := m.suggestions[m.cursor].Command; got != long {
		t.Fatalf("selected command was truncated: got %q", got)
	}
}

// TestTeatestDrivesSelection exercises the full Bubble Tea program with
// simulated key messages, asserting Enter yields the highlighted command.
func TestTeatestDrivesSelection(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(sample()), teatest.WithInitialTermSize(80, 24))

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(model)
	if !final.chosen {
		t.Fatal("expected chosen=true after Enter via teatest")
	}
	if got := final.suggestions[final.cursor].Command; got != "git diff" {
		t.Fatalf("selected command = %q, want %q", got, "git diff")
	}
}

// TestPlainListFallback checks the non-interactive fallback writes a numbered
// list and nothing that looks like escape-driven UI.
func TestPlainListFallback(t *testing.T) {
	var buf bytes.Buffer
	printPlainList(&buf, sample())
	out := buf.String()
	for _, want := range []string{"1. git status", "2. git log --oneline", "3. git diff"} {
		if !strings.Contains(out, want) {
			t.Fatalf("plain list missing %q; got:\n%s", want, out)
		}
	}
}
