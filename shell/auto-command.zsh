# auto-command zsh integration.
#
# Source this file (or `eval "$(acmd init zsh)"`) from your ~/.zshrc. Type a
# natural-language request at the prompt, then press Ctrl-G. auto-command asks
# for a shell command and, if you pick one, places it in the prompt buffer for
# you to review and run. Nothing is ever executed automatically — you press
# Enter yourself.

_auto_command_widget() {
  # Use the current buffer as the query. `acmd -p` prints the chosen command to
  # stdout instead of running it, so the widget can place it in the prompt
  # buffer for review. `acmd` prints usage to stderr and exits non-zero on an
  # empty query, so an empty buffer leaves the prompt untouched.
  local cmd
  cmd=$(acmd -p "$BUFFER")
  if [[ $? -eq 0 && -n $cmd ]]; then
    # Replace the prompt buffer with the chosen command, unexecuted. Setting
    # BUFFER/CURSOR keeps special characters verbatim and never submits.
    BUFFER=$cmd
    CURSOR=${#BUFFER}
  fi
  # Repaint the prompt (the picker took over the terminal while it ran).
  zle reset-prompt
}

zle -N _auto_command_widget
# Bound to Ctrl-G by default; rebind by copying this line with a different key.
bindkey '^G' _auto_command_widget
