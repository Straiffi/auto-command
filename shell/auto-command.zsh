# auto-command zsh integration.
#
# Source this file (or `eval "$(acmd init zsh)"`) from your ~/.zshrc. Type a
# natural-language request at the prompt, then press Ctrl-G. auto-command asks
# for a shell command and, if you pick one, runs and prints it — mirroring the
# default `acmd "<query>"` behavior. The chosen command is placed on the prompt
# line, executed in the current shell, and recorded in your interactive history
# so you can recall it with up-arrow or Ctrl+R.

_auto_command_widget() {
  # Use the current buffer as the query. `acmd -p` prints the chosen command to
  # stdout instead of running it, so the widget can capture it. `acmd` prints
  # usage to stderr and exits non-zero on an empty query, so an empty buffer
  # leaves the prompt untouched.
  local cmd
  cmd=$(acmd -p "$BUFFER")
  if [[ $? -eq 0 && -n $cmd ]]; then
    # Put the chosen command on the prompt line and submit it. Setting
    # BUFFER/CURSOR keeps special characters verbatim; `accept-line` makes zsh
    # print the command, run it in the current shell, and record it in history.
    BUFFER=$cmd
    CURSOR=${#BUFFER}
    zle accept-line
  else
    # Empty query or cancelled picker: change nothing and repaint the prompt
    # (the picker took over the terminal while it ran).
    zle reset-prompt
  fi
}

zle -N _auto_command_widget
# Bound to Ctrl-G by default; rebind by copying this line with a different key.
bindkey '^G' _auto_command_widget
