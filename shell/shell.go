// Package shell embeds the shell integration snippets so the CLI and the
// checked-in scripts share a single source and cannot drift.
package shell

import _ "embed"

// Zsh is the zsh ZLE widget printed by `ac init zsh`. It is embedded from
// shell/auto-command.zsh, the same file users can source directly.
//
//go:embed auto-command.zsh
var Zsh string
