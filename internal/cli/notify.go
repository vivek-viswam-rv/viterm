package cli

import (
	"io"
	"os"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func handleNotify(args []string, stderr io.Writer) int {
	color := protocol.ColorGreen
	if len(args) >= 1 {
		color = args[0]
	}

	socket, ok := requireSocket(stderr)
	if !ok {
		return 1
	}

	cmd := protocol.Command{
		Action:  protocol.ActionSessionNotify,
		PaneID:  os.Getenv(protocol.EnvPaneID),
		Command: color,
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
