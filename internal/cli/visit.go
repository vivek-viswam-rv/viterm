package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func handleVisit(args []string, stderr io.Writer) int {
	positional, flags, boolFlags := parseFlags(args)
	if len(positional) < 1 {
		fmt.Fprintln(stderr, "Usage: viterm visit <url> [--pane N] [--background]")
		return 1
	}

	socket, ok := requireSocket(stderr)
	if !ok {
		return 1
	}

	cmd := protocol.Command{
		Action: protocol.ActionURLOpen,
		PaneID: os.Getenv(protocol.EnvPaneID),
		URL:    strings.Join(positional, " "),
	}
	if v, present := flags["pane"]; present {
		n, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintf(stderr, "Error: invalid --pane value %q\n", v)
			return 1
		}
		cmd.PaneSeq = &n
	}
	if boolFlags["background"] {
		bg := true
		cmd.Background = &bg
	}

	reply, ok := sendOrFail(socket, cmd, stderr)
	if !ok {
		return 1
	}
	if !checkReply(reply, stderr) {
		return 1
	}
	return 0
}
