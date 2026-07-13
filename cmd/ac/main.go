// Command ac is the auto-command CLI. It dispatches to subcommands or treats
// its arguments as a natural-language query.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/straiffi/auto-command/internal/app"
	"github.com/straiffi/auto-command/internal/config"
	"github.com/straiffi/auto-command/shell"
)

const usage = `usage:
  ac "<query>"        ask for a shell command
  ac config           create/show the config file path
  ac init zsh         print zsh shell integration
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	switch args[0] {
	case "config":
		return runConfig()
	case "init":
		return runInit(args[1:])
	default:
		return runQuery(args)
	}
}

func runConfig() int {
	path, created, err := config.EnsureTemplate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ac: "+err.Error())
		return 1
	}
	if created {
		fmt.Printf("created %s\n", path)
	} else {
		fmt.Println(path)
	}
	return 0
}

func runInit(args []string) int {
	if len(args) == 0 || args[0] != "zsh" {
		fmt.Fprintln(os.Stderr, "usage: ac init zsh")
		return 2
	}
	// Print the embedded widget verbatim so `eval "$(ac init zsh)"` and
	// sourcing shell/auto-command.zsh stay identical.
	fmt.Print(shell.Zsh)
	return 0
}

func runQuery(args []string) int {
	outcome := app.Run(context.Background(), app.Options{
		Query: strings.Join(args, " "),
	})
	return exitCode(outcome)
}

// exitCode maps a run outcome to a process exit code: 0 when a command was
// emitted, 2 for a usage error, and 1 for everything else (missing key,
// cancellation, no suggestions, or any failure).
func exitCode(o app.Outcome) int {
	switch o {
	case app.Success:
		return 0
	case app.InvalidQuery:
		return 2
	default:
		return 1
	}
}
