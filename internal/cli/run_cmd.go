package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func handleRun(args []string, stderr io.Writer) int {
	var paneSeq *int
	if len(args) >= 2 && args[0] == "--pane" {
		n, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "Error: invalid --pane value %q\n", args[1])
			return 1
		}
		paneSeq = &n
		args = args[2:]
	}

	if len(args) < 1 {
		fmt.Fprintln(stderr, "Usage: viterm run [--pane N] <command...>")
		return 1
	}

	socket, ok := requireSocket(stderr)
	if !ok {
		return 1
	}

	cmd := protocol.Command{
		Action:  protocol.ActionTerminalRun,
		PaneID:  os.Getenv(protocol.EnvPaneID),
		PaneSeq: paneSeq,
		Command: strings.Join(args, " "),
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
