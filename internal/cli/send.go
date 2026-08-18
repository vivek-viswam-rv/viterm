package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func handleSend(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: viterm send <tab-id> <text...>")
		return 1
	}

	socket, ok := requireSocket(stderr)
	if !ok {
		return 1
	}

	tabID := args[0]
	text := unescapeText(strings.Join(args[1:], " "))

	cmd := protocol.Command{
		Action: protocol.ActionTabSend,
		PaneID: os.Getenv(protocol.EnvPaneID),
		TabID:  tabID,
		Text:   text,
	}
	reply, ok := sendOrFail(socket, cmd, stderr)
	if !ok {
		return 1
	}

	var r protocol.Reply
	if err := json.Unmarshal(reply, &r); err != nil {
		fmt.Fprintln(stderr, "Error: no response from viterm")
		return 1
	}
	if !r.OK {
		msg := r.Error
		if msg == "" {
			msg = "unknown error"
		}
		fmt.Fprintf(stderr, "Error: %s\n", msg)
		return 1
	}

	fmt.Fprintf(stdout, "Sent to %s\n", tabID)
	return 0
}

// unescapeText converts literal two-character escape sequences typed on the
// command line ("\n", "\t") into real newline and tab bytes.
func unescapeText(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}
