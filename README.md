# auto-command (`ac`)

Turn a plain-English request into a shell command. Type what you want, press a
key, and the suggested command lands in your zsh prompt — **ready for you to
review and run**. auto-command never executes anything for you; you always press
Enter yourself.

## Install

### With `go install`

```sh
go install github.com/juuso/auto-command/cmd/ac@latest
```

Or from a checkout of this repository:

```sh
go install ./cmd/ac
```

This puts an `ac` binary in `$(go env GOPATH)/bin` (make sure that's on your
`PATH`).

### Prebuilt binary

Grab a release archive for your platform from the
[Releases](https://github.com/juuso/auto-command/releases) page. Statically
linked binaries are published for:

- macOS (`darwin`) amd64 / arm64
- Linux amd64 / arm64

```sh
# Example: Linux amd64
tar -xzf auto-command_*_linux_amd64.tar.gz
sudo mv ac /usr/local/bin/
ac           # run with no query to print usage
```

## Configure

auto-command talks to [OpenRouter](https://openrouter.ai). Create the config
file and set your key and model:

```sh
ac config          # creates ~/.config/auto-command/config.toml (0600) and prints its path
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
eval "$(ac init zsh)"
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
4. The chosen command appears in your prompt buffer, **unexecuted**. Review it,
   edit if you like, and press Enter to run it yourself.

If the buffer is empty when you press Ctrl-G, or you cancel the picker, the
prompt is left unchanged. **Commands are never auto-executed.**

To bind a different key, copy the `bindkey` line from
`shell/auto-command.zsh` and change `^G`.

## Usage without the widget

```sh
ac "list all git branches merged into main"
```

`ac` prints the chosen command to stdout (and only that), so it composes with
other tools. All status and error output goes to stderr.

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
