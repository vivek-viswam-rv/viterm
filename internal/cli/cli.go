// Package cli implements the viterm command-line companion tool: the client
// side of the internal/ipc transport, driven from a shell inside a running
// viterm session (or, for install-hooks, independent of any session).
package cli

import (
	"fmt"
	"io"
	"os"
)

// Run executes the viterm CLI with the given arguments (excluding the
// program name) and returns the process exit code.
func Run(args []string, version string) int {
	return run(args, version, os.Stdin, os.Stdout, os.Stderr)
}

// run is the testable core of Run: it takes explicit stdin/stdout/stderr so
// tests can capture output and supply canned input without touching the
// real process streams.
func run(args []string, version string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return 0

	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return 0

	case "tabs":
		return handleTabs(stdout, stderr)

	case "send":
		return handleSend(args[1:], stdout, stderr)

	case "run":
		return handleRun(args[1:], stderr)

	case "visit":
		return handleVisit(args[1:], stderr)

	case "browser":
		if len(args) < 2 || args[1] != "open" {
			fmt.Fprintln(stderr, "Usage: viterm browser open <url> [--pane N] [--background]")
			return 1
		}
		return handleVisit(args[2:], stderr)

	case "notify":
		return handleNotify(args[1:], stderr)

	case "claude-event":
		return handleClaudeEvent(args[1:], stdin, stderr)

	case "install-hooks":
		dryRun := false
		for _, a := range args[1:] {
			if a == "--dry-run" {
				dryRun = true
			}
		}
		return installHooks(dryRun, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, usageText)
		return 2
	}
}

const usageText = `Usage: viterm <command> [args]

Commands:
  tabs                                           List all tabs
  send <tab-id> <text...>                        Send text to a terminal tab
  run [--pane N] <command...>                    Open a terminal tab running a command
  visit <url> [--pane N] [--background]          Open a browser tab
  browser open <url> [--pane N] [--background]   Alias for visit
  notify [color]                                 Set session tab color (green, red, yellow, blue, orange, clear)
  claude-event <kind>                            Forward a Claude Code hook event (used internally by installed hooks)
  install-hooks [--dry-run]                      Install Claude Code hooks into ~/.claude/settings.json
  help                                           Show this help text
  version                                        Print the viterm version

Environment:
  VITERM_SOCKET   Path to the session's control socket (set automatically inside a viterm pane)
  VITERM_PANE_ID  ID of the pane the command is running in (set automatically inside a viterm pane)
`
