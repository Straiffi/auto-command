package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/straiffi/auto-command/internal/config"
	"github.com/straiffi/auto-command/internal/llm"
	"github.com/straiffi/auto-command/internal/ui"
)

// fakeClient is a scripted LLMClient. It records that it was constructed/called
// so tests can assert whether the network path was reached.
type fakeClient struct {
	suggestions []llm.Suggestion
	err         error
	called      *bool
}

func (c fakeClient) Suggest(ctx context.Context, query string) ([]llm.Suggestion, error) {
	if c.called != nil {
		*c.called = true
	}
	return c.suggestions, c.err
}

// scriptedPicker returns a fixed selection without touching a terminal.
type scriptedPicker struct {
	command  string
	selected bool
	err      error
	got      *[]ui.Suggestion
}

func (p scriptedPicker) Pick(s []ui.Suggestion) (string, bool, error) {
	if p.got != nil {
		*p.got = s
	}
	return p.command, p.selected, p.err
}

func okConfig() (*config.Config, error) {
	return &config.Config{APIKey: "test-key", Model: "test/model", MaxSuggestions: 3}, nil
}

func newClientFactory(c LLMClient) func(*config.Config) LLMClient {
	return func(*config.Config) LLMClient { return c }
}

// run is a small helper that wires up buffers and runs the flow.
func run(t *testing.T, opts Options) (Outcome, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	if opts.LoadConfig == nil {
		opts.LoadConfig = okConfig
	}
	out := Run(context.Background(), opts)
	return out, stdout.String(), stderr.String()
}

func TestSelectedCommandReachesStdout(t *testing.T) {
	suggestions := []llm.Suggestion{
		{Command: "du -sh *", Explanation: "size of each entry"},
		{Command: "ls -laS", Explanation: "list files by size"},
	}
	var picked []ui.Suggestion
	outcome, stdout, stderr := run(t, Options{
		Query:     "list files by size",
		NewClient: newClientFactory(fakeClient{suggestions: suggestions}),
		Picker:    scriptedPicker{command: "ls -laS", selected: true, got: &picked},
	})

	if outcome != Success {
		t.Fatalf("outcome = %v, want Success", outcome)
	}
	if got := strings.TrimSpace(stdout); got != "ls -laS" {
		t.Fatalf("stdout = %q, want %q", got, "ls -laS")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on success", stderr)
	}
	if len(picked) != 2 || picked[1].Command != "ls -laS" {
		t.Fatalf("picker received %+v, want the two converted suggestions", picked)
	}
}

func TestMissingKeySkipsNetworkAndWritesStderr(t *testing.T) {
	called := false
	outcome, stdout, stderr := run(t, Options{
		Query:      "list files",
		LoadConfig: func() (*config.Config, error) { return &config.Config{}, &config.MissingError{Fields: []string{"api_key"}} },
		NewClient:  newClientFactory(fakeClient{called: &called}),
		Picker:     scriptedPicker{},
	})

	if outcome != MissingKey {
		t.Fatalf("outcome = %v, want MissingKey", outcome)
	}
	if called {
		t.Fatal("client was constructed/called despite missing key; no network call should happen")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "acmd config") {
		t.Fatalf("stderr = %q, want a hint pointing to 'acmd config'", stderr)
	}
}

func TestAPIErrorWritesOnlyStderr(t *testing.T) {
	apiErr := &llm.APIError{StatusCode: 429, Status: "429 Too Many Requests", Message: "rate limited"}
	outcome, stdout, stderr := run(t, Options{
		Query:     "list files",
		NewClient: newClientFactory(fakeClient{err: apiErr}),
		Picker:    scriptedPicker{},
	})

	if outcome != Failed {
		t.Fatalf("outcome = %v, want Failed", outcome)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on error", stdout)
	}
	if !strings.Contains(stderr, "429") {
		t.Fatalf("stderr = %q, want the status code included", stderr)
	}
}

func TestTimeoutWritesOnlyStderr(t *testing.T) {
	outcome, stdout, stderr := run(t, Options{
		Query:     "list files",
		NewClient: newClientFactory(fakeClient{err: errors.New("contacting OpenRouter: context deadline exceeded")}),
		Picker:    scriptedPicker{},
	})

	if outcome != Failed {
		t.Fatalf("outcome = %v, want Failed", outcome)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on timeout", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Fatal("expected a message on stderr for a timeout")
	}
}

func TestNoSuggestionsWritesFriendlyStderr(t *testing.T) {
	outcome, stdout, stderr := run(t, Options{
		Query:     "list files",
		NewClient: newClientFactory(fakeClient{err: llm.ErrNoSuggestions}),
		Picker:    scriptedPicker{},
	})

	if outcome != NoSuggestions {
		t.Fatalf("outcome = %v, want NoSuggestions", outcome)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no command suggestions") {
		t.Fatalf("stderr = %q, want a friendly no-suggestions message", stderr)
	}
}

func TestCancellationLeavesStdoutEmpty(t *testing.T) {
	outcome, stdout, stderr := run(t, Options{
		Query:     "list files",
		NewClient: newClientFactory(fakeClient{suggestions: []llm.Suggestion{{Command: "ls"}}}),
		Picker:    scriptedPicker{selected: false},
	})

	if outcome != Cancelled {
		t.Fatalf("outcome = %v, want Cancelled", outcome)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty when the user cancels", stdout)
	}
	// Cancellation is a quiet path: nothing is required on stderr either.
	_ = stderr
}

func TestWhitespaceQuerySkipsNetworkAndPrintsUsage(t *testing.T) {
	called := false
	outcome, stdout, stderr := run(t, Options{
		Query:     "   \t  ",
		NewClient: newClientFactory(fakeClient{called: &called}),
		Picker:    scriptedPicker{},
	})

	if outcome != InvalidQuery {
		t.Fatalf("outcome = %v, want InvalidQuery", outcome)
	}
	if called {
		t.Fatal("client was called for a whitespace-only query")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Fatalf("stderr = %q, want usage text", stderr)
	}
}
