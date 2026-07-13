// Package app composes the configuration loader, the OpenRouter client, and the
// interactive picker into the default `ac "<query>"` flow. It owns the ordering
// of checks (config and key validation before any network call), routes every
// human-facing status and error message to stderr, and reserves stdout for the
// single chosen command so callers such as a shell integration can capture it
// cleanly.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/straiffi/auto-command/internal/config"
	"github.com/straiffi/auto-command/internal/llm"
	"github.com/straiffi/auto-command/internal/ui"

	"github.com/mattn/go-isatty"
)

// usage is printed to stderr when the query is empty or whitespace-only.
const usage = `usage: ac "<query>"   ask for a shell command`

// Outcome is the typed result of Run. main maps it to a process exit code; the
// package itself never calls os.Exit so it stays testable.
type Outcome int

const (
	// Success means a command was chosen and written to stdout.
	Success Outcome = iota
	// InvalidQuery means the query was empty or whitespace; usage was printed.
	InvalidQuery
	// MissingKey means no API key was configured; no network call was made.
	MissingKey
	// NoSuggestions means the model returned an empty suggestion set.
	NoSuggestions
	// Cancelled means the user aborted the picker without choosing.
	Cancelled
	// Failed means an error occurred (config, network, API, or picker).
	Failed
)

// LLMClient produces command suggestions for a natural-language query. It is an
// interface so Run can be exercised with a fake in tests; *llm.Client satisfies
// it directly.
type LLMClient interface {
	Suggest(ctx context.Context, query string) ([]llm.Suggestion, error)
}

// Picker presents suggestions and reports the chosen command. selected is false
// when the user cancels. It is an interface so tests can script a selection
// without a terminal.
type Picker interface {
	Pick(suggestions []ui.Suggestion) (command string, selected bool, err error)
}

// Options carries the query plus the collaborators Run depends on. Every
// collaborator is optional: unset fields fall back to the real implementations,
// while tests inject fakes. This is the dependency-injection seam described in
// the ticket.
type Options struct {
	// Query is the raw natural-language request (before trimming).
	Query string
	// LoadConfig resolves configuration. Defaults to config.Load.
	LoadConfig func() (*config.Config, error)
	// NewClient builds an LLM client from the resolved config. It is only
	// called after the key check passes, so a fake here proves no network call
	// happens on the missing-key path. Defaults to a real *llm.Client.
	NewClient func(*config.Config) LLMClient
	// Picker runs the interactive selection. Defaults to the real ui picker.
	Picker Picker
	// Stdout receives the chosen command only. Defaults to os.Stdout.
	Stdout io.Writer
	// Stderr receives all status and error output. Defaults to os.Stderr.
	Stderr io.Writer
}

// defaultPicker adapts ui.Pick to the Picker interface.
type defaultPicker struct{}

func (defaultPicker) Pick(s []ui.Suggestion) (string, bool, error) { return ui.Pick(s) }

// Run executes the default query flow and returns a typed Outcome. It writes
// nothing to stdout on any non-success path.
func Run(ctx context.Context, opts Options) Outcome {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	loadConfig := opts.LoadConfig
	if loadConfig == nil {
		loadConfig = config.Load
	}
	newClient := opts.NewClient
	if newClient == nil {
		newClient = func(cfg *config.Config) LLMClient { return llm.New(cfg) }
	}
	picker := opts.Picker
	if picker == nil {
		picker = defaultPicker{}
	}

	query := strings.TrimSpace(opts.Query)
	if query == "" {
		fmt.Fprintln(stderr, usage)
		return InvalidQuery
	}

	// Config and key validation happen before any network call.
	cfg, err := loadConfig()
	if err != nil {
		var missing *config.MissingError
		if errors.As(err, &missing) {
			fmt.Fprintln(stderr, "ac: no API key configured. Set AUTO_COMMAND_API_KEY (or OPENROUTER_API_KEY), or run 'ac config' to create a config file.")
			return MissingKey
		}
		fmt.Fprintln(stderr, "ac: "+err.Error())
		return Failed
	}

	client := newClient(cfg)

	// Show a loading indicator on the terminal while the request is in flight.
	// It writes to stderr only and clears itself when the request returns.
	stopSpinner := startSpinner(stderr)
	suggestions, err := client.Suggest(ctx, query)
	stopSpinner()

	if err != nil {
		switch {
		case errors.Is(err, llm.ErrNoSuggestions):
			fmt.Fprintln(stderr, "ac: no command suggestions for that query; try rephrasing.")
			return NoSuggestions
		default:
			// APIError.Error() already carries the status and any provider
			// message; other errors are wrapped into short messages upstream.
			fmt.Fprintln(stderr, "ac: "+err.Error())
			return Failed
		}
	}

	// Defensive: a well-behaved client returns ErrNoSuggestions, but never
	// hand an empty set to the picker.
	if len(suggestions) == 0 {
		fmt.Fprintln(stderr, "ac: no command suggestions for that query; try rephrasing.")
		return NoSuggestions
	}

	command, selected, err := picker.Pick(toUISuggestions(suggestions))
	if err != nil {
		fmt.Fprintln(stderr, "ac: "+err.Error())
		return Failed
	}
	if !selected {
		return Cancelled
	}

	fmt.Fprintln(stdout, command)
	return Success
}

// toUISuggestions converts the llm suggestion shape to the ui shape. The two
// packages keep independent types so neither depends on the other.
func toUISuggestions(in []llm.Suggestion) []ui.Suggestion {
	out := make([]ui.Suggestion, len(in))
	for i, s := range in {
		out[i] = ui.Suggestion{Command: s.Command, Explanation: s.Explanation}
	}
	return out
}

// spinnerFrames is a small braille animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// startSpinner animates a "thinking" indicator on w and returns a function that
// stops it and clears the line. It only animates when w is a real terminal, so
// redirected output (and tests using a buffer) stay silent. The indicator never
// touches stdout.
func startSpinner(w io.Writer) (stop func()) {
	f, ok := w.(*os.File)
	if !ok || !(isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())) {
		return func() {}
	}

	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-done:
				// Clear the line so the spinner leaves no residue.
				fmt.Fprint(f, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(f, "\r%s thinking…", spinnerFrames[i%len(spinnerFrames)])
				i++
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}
