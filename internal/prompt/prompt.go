// Package prompt builds the messages sent to the model: a system prompt that
// pins the output format and embeds the minimal environment context, and a user
// message carrying the raw query verbatim.
package prompt

import (
	"fmt"
	"strings"

	"github.com/juuso/auto-command/internal/envctx"
)

// Message is a single chat message. Role is one of "system" or "user".
type Message struct {
	Role    string
	Content string
}

// Build returns the system and user messages for a query. maxSuggestions bounds
// how many suggestions the model is asked to return; the raw query is placed in
// the user message unmodified so quoting and newlines survive intact.
func Build(env envctx.Context, query string, maxSuggestions int) []Message {
	return []Message{
		{Role: "system", Content: systemPrompt(env, maxSuggestions)},
		{Role: "user", Content: query},
	}
}

// systemPrompt describes the task, the strict JSON contract, and the
// environment. It intentionally includes only the OS/arch/shell — no cwd,
// listings, environment variables, or history.
func systemPrompt(env envctx.Context, maxSuggestions int) string {
	if maxSuggestions < 1 {
		maxSuggestions = 1
	}

	var b strings.Builder
	b.WriteString("You are a command-line assistant. Given a natural-language request, ")
	b.WriteString("suggest shell commands that accomplish it.\n\n")

	b.WriteString("Environment:\n")
	fmt.Fprintf(&b, "- Operating system: %s\n", env.OS)
	fmt.Fprintf(&b, "- Architecture: %s\n", env.Arch)
	fmt.Fprintf(&b, "- Shell: %s\n\n", env.Shell)

	b.WriteString("Rules:\n")
	fmt.Fprintf(&b, "- Return between 1 and %d suggestions, ordered best first.\n", maxSuggestions)
	b.WriteString("- Each suggestion must be a single, ready-to-run command for the shell above.\n")
	b.WriteString("- Keep explanations to one concise sentence.\n")
	b.WriteString("- Respond with strict JSON only, no markdown, no prose outside the JSON.\n\n")

	b.WriteString("Respond with exactly this JSON shape:\n")
	b.WriteString(`{"suggestions":[{"command":"...","explanation":"..."}]}`)
	b.WriteString("\n")

	return b.String()
}
