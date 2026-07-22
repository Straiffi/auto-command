# auto-command (`acmd`)

Turn a plain-English request into a shell command. Type what you want, pick a
suggestion, and **the command you choose runs**. Selecting it in the picker
(pressing Enter) is the confirmation step — nothing runs until you pick it, and
if you cancel, nothing runs at all.

The Ctrl-G zsh widget mirrors this: it prints the chosen command on your prompt
line and runs it in the current shell, recording it in your interactive history.
Prefer to review before running? Use `acmd -p` (print mode), which writes the
chosen command to stdout without executing it, so you can inspect or compose it.

> The command is `acmd` (not `ac`) to avoid clashing with the GNU accounting
> `ac` tool that ships pre-installed on many distros.

## Install

### With `go install`

```sh
go install github.com/straiffi/auto-command/cmd/acmd@latest
```

Or from a checkout of this repository:

```sh
go install ./cmd/acmd
```

This puts an `acmd` binary in `$(go env GOPATH)/bin` (make sure that's on your
`PATH`).

### Prebuilt binary

Grab a release archive for your platform from the
[Releases](https://github.com/straiffi/auto-command/releases) page. Statically
linked binaries are published for:

- macOS (`darwin`) amd64 / arm64
- Linux amd64 / arm64

```sh
# Example: Linux amd64
tar -xzf auto-command_*_linux_amd64.tar.gz
sudo mv acmd /usr/local/bin/
acmd         # run with no query to print usage
```

## Configure

auto-command talks to [OpenRouter](https://openrouter.ai). Create the config
file and set your key and model:

```sh
acmd config        # creates ~/.config/auto-command/config.toml (0600) and prints its path
```

Edit the file to set your OpenRouter API key and model:

```toml
api_key = "sk-or-..."
model   = "openai/gpt-4o-mini"
max_suggestions = 3
```

Prefer keeping the key out of a file? Set it in the environment instead
(takes precedence over the config file):

```sh
export AUTO_COMMAND_API_KEY="sk-or-..."   # or OPENROUTER_API_KEY
export AUTO_COMMAND_MODEL="openai/gpt-4o-mini"   # optional model override
```

Config file location follows `XDG_CONFIG_HOME`, defaulting to
`~/.config/auto-command/config.toml`.

## Enable the zsh widget

Add one line to your `~/.zshrc`:

```sh
eval "$(acmd init zsh)"
```

Or source the checked-in script directly (identical content):

```sh
source /path/to/auto-command/shell/auto-command.zsh
```

Reload your shell, then:

1. Type a natural-language request at the prompt, e.g.
   `find files larger than 100MB under this directory`.
2. Press **Ctrl-G**.
3. Pick a suggestion in the interactive picker.
4. The chosen command is printed on your prompt line and **run** in the current
   shell — mirroring default `acmd` behavior. It also lands in your interactive
   history, so you can recall it with up-arrow or Ctrl+R.

If the buffer is empty when you press Ctrl-G, or you cancel the picker, the
prompt is left unchanged and nothing runs. Because the widget runs the command
via `zle accept-line` in the current shell, shell-state changes such as `cd`,
`export`, or `source` do affect your session.

To bind a different key, copy the `bindkey` line from
`shell/auto-command.zsh` and change `^G`.

## Usage without the widget

```sh
acmd "list all git branches merged into main"   # runs the command you pick
```

By default `acmd` runs the command you select in your shell (`$SHELL -c`,
falling back to `/bin/sh`); its output goes straight to your terminal and
`acmd` exits with the command's own exit code.

Note that a command selected this way runs in a subprocess, so shell-state
changes such as `cd`, `export`, or `source` do not affect your current shell —
use the Ctrl-G widget (which runs the command in the current shell) or print
mode for those.

Pass `-p` (or `--print`) to print the chosen command to stdout instead of
running it, so it composes with other tools:

```sh
eval "$(acmd -p 'list all git branches merged into main')"
```

All status and error output goes to stderr.

## Building and releasing

```sh
go build ./...                              # build everything
goreleaser release --snapshot --clean       # local, unpublished build of all targets
```

A tagged release is produced with:

```sh
git tag v0.1.0
git push --tags
goreleaser release --clean
```
