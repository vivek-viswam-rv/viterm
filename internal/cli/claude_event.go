package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/vivek-viswam-rv/viterm/internal/ipc"
	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

// claudeHookPayload is the small subset of a Claude Code hook's stdin JSON
// payload viterm cares about. Parsing is tolerant: hooks may send an empty
// body, extra fields, or (in principle) malformed JSON, and none of that
// should ever cause the hook itself to fail.
type claudeHookPayload struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
}

// handleClaudeEvent forwards a Claude Code hook event to the running viterm
// session. It must never make a hook fail: outside a viterm session (no
// VITERM_SOCKET) or when the session's socket is gone, it exits 0 silently.
func handleClaudeEvent(args []string, stdin io.Reader, stderr io.Writer) int {
	socket := os.Getenv(protocol.EnvSocket)
	if socket == "" {
		return 0
	}

	if len(args) < 1 {
		fmt.Fprintln(stderr, "Usage: viterm claude-event <kind>")
		return 1
	}
	kind := args[0]

	body, _ := io.ReadAll(stdin)
	var payload claudeHookPayload
	_ = json.Unmarshal(body, &payload) // tolerant of empty/malformed stdin

	cmd := protocol.Command{
		Action:          protocol.ActionClaudeEvent,
		PaneID:          os.Getenv(protocol.EnvPaneID),
		Command:         kind,
		ClaudeSessionID: payload.SessionID,
		CWD:             payload.CWD,
	}

	if _, err := ipc.Send(socket, cmd); err != nil {
		// The session's socket is gone (app closed, etc). Hooks must stay
		// silent no-ops in that case too.
		return 0
	}
	return 0
}
