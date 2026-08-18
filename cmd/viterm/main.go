// Command viterm is a terminal workspace manager for AI coding agents. Run
// it with no arguments to open the full-screen application; with a
// subcommand it acts as a client controlling the session it runs inside.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/vivek-viswam-rv/viterm/internal/cli"
	"github.com/vivek-viswam-rv/viterm/internal/config"
	"github.com/vivek-viswam-rv/viterm/internal/ui"
)

// version is set at build time.
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		os.Exit(cli.Run(os.Args[1:], version))
	}

	store, err := config.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "viterm:", err)
		os.Exit(1)
	}
	app, err := ui.New(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "viterm:", err)
		os.Exit(1)
	}
	program := tea.NewProgram(app)
	app.SetProgram(program)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "viterm:", err)
		os.Exit(1)
	}
}
