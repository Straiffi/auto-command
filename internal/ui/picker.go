// Package ui renders the interactive command picker. The picker only ever
// selects and returns text; it never executes a command, and it never writes
// UI output to stdout — stdout is reserved for the single chosen command so
// callers (such as a shell integration) can capture it cleanly.
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
)

// Suggestion is a single candidate command paired with a one-line explanation.
type Suggestion struct {
	// Command is the full shell command. It is returned verbatim when chosen,
	// even if the display is truncated for width.
	Command string
	// Explanation is a short, one-line description of what the command does.
	Explanation string
}

var (
	cursorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	selectedCmdStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))
	explanationStyle = lipgloss.NewStyle().Faint(true)
	helpStyle        = lipgloss.NewStyle().Faint(true)
)

// model is the Bubble Tea model backing the picker.
type model struct {
	suggestions []Suggestion
	cursor      int
	// chosen is set when the user confirmed a selection with Enter.
	chosen bool
	// cancelled is set when the user aborted with Esc or Ctrl-C.
	cancelled bool
	width     int
}

func newModel(suggestions []Suggestion) model {
	return model{suggestions: suggestions, width: 80}
}

func (m model) Init() tea.Cmd { return nil }

// Update handles navigation and confirmation keys. A single suggestion still
// requires an explicit Enter — there is no auto-select.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.suggestions)-1 {
				m.cursor++
			}
		case "enter":
			m.chosen = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	// Once the program is quitting there is nothing to draw; returning empty
	// keeps the alternate screen from flashing stale content on exit.
	if m.chosen || m.cancelled {
		return ""
	}

	width := m.width
	if width <= 0 {
		width = 80
	}
	// Reserve two columns for the cursor gutter; keep a sane floor so very
	// narrow terminals still show something selectable.
	avail := width - 2
	if avail < 10 {
		avail = 10
	}

	var b strings.Builder
	b.WriteString("Select a command:\n\n")
	for i, s := range m.suggestions {
		gutter := "  "
		cmd := runewidth.Truncate(s.Command, avail, "…")
		if i == m.cursor {
			gutter = cursorStyle.Render("> ")
			cmd = selectedCmdStyle.Render(cmd)
		}
		b.WriteString(gutter + cmd + "\n")
		if s.Explanation != "" {
			exp := runewidth.Truncate(s.Explanation, avail, "…")
			b.WriteString("  " + explanationStyle.Render(exp) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · esc cancel"))
	return b.String()
}

// Pick presents the suggestions on the controlling terminal and blocks until
// the user confirms or cancels. It renders to /dev/tty (falling back to stderr)
// so stdout is never touched. It returns the chosen command and selected=true
// on Enter; on Esc/Ctrl-C it returns selected=false.
//
// When no interactive terminal is available, it prints the suggestions as a
// plain numbered list to stderr and returns selected=false rather than hanging
// on input that will never arrive.
func Pick(suggestions []Suggestion) (command string, selected bool, err error) {
	if len(suggestions) == 0 {
		return "", false, errors.New("no suggestions to pick from")
	}

	in, out, interactive, cleanup := openTTY()
	defer cleanup()

	if !interactive {
		printPlainList(os.Stderr, suggestions)
		return "", false, nil
	}

	p := tea.NewProgram(
		newModel(suggestions),
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)
	final, err := p.Run()
	if err != nil {
		return "", false, err
	}

	fm := final.(model)
	if fm.chosen {
		return fm.suggestions[fm.cursor].Command, true, nil
	}
	return "", false, nil
}

// Run is the caller-facing helper: it drives the picker and writes ONLY the
// chosen command (with a single trailing newline) to stdout, returning 0 when a
// command was selected. When cancelled or on error it writes nothing to stdout
// and returns a non-zero exit code.
func Run(suggestions []Suggestion) int {
	command, selected, err := Pick(suggestions)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ac: "+err.Error())
		return 1
	}
	if !selected {
		return 1
	}
	fmt.Fprintln(os.Stdout, command)
	return 0
}

// openTTY resolves the input/output for the picker. It prefers the controlling
// terminal (/dev/tty) so the UI works even when stdin/stdout are redirected
// (e.g. shell command substitution) and never leaks onto stdout. If /dev/tty
// cannot be opened it falls back to reading stdin and rendering to stderr, but
// only when stderr is a real terminal. Otherwise interactive is false.
func openTTY() (in io.Reader, out io.Writer, interactive bool, cleanup func()) {
	cleanup = func() {}

	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		return tty, tty, true, func() { _ = tty.Close() }
	}

	if isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		return os.Stdin, os.Stderr, true, cleanup
	}

	return nil, nil, false, cleanup
}

func printPlainList(w io.Writer, suggestions []Suggestion) {
	fmt.Fprintln(w, "ac: no interactive terminal available; suggested commands:")
	for i, s := range suggestions {
		fmt.Fprintf(w, "  %d. %s\n", i+1, s.Command)
		if s.Explanation != "" {
			fmt.Fprintf(w, "     %s\n", s.Explanation)
		}
	}
}
