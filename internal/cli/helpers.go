package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vivekviswam/viterm/internal/ipc"
	"github.com/vivekviswam/viterm/internal/protocol"
)

// requireSocket reads VITERM_SOCKET and reports a clear error if it isn't
// set. Every socket-backed command except claude-event (which must stay
// silent outside a session) goes through this.
func requireSocket(stderr io.Writer) (string, bool) {
	socket := os.Getenv(protocol.EnvSocket)
	if socket == "" {
		fmt.Fprintln(stderr, "VITERM_SOCKET is not set; run this command inside a viterm session")
		return "", false
	}
	return socket, true
}

// sendOrFail sends cmd over socket, printing a transport-level error and
// reporting failure on any error (connection refused, timeout, etc).
func sendOrFail(socket string, cmd protocol.Command, stderr io.Writer) ([]byte, bool) {
	reply, err := ipc.Send(socket, cmd)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return nil, false
	}
	return reply, true
}

// checkReply inspects a reply for the generic {ok, error} shape used by
// actions that don't return a richer payload (tabs.list's JSON array simply
// won't unmarshal into a Reply and is treated as "nothing to check" here).
func checkReply(reply []byte, stderr io.Writer) bool {
	if len(reply) == 0 {
		return true
	}
	var r protocol.Reply
	if err := json.Unmarshal(reply, &r); err != nil {
		return true
	}
	if !r.OK {
		msg := r.Error
		if msg == "" {
			msg = "unknown error"
		}
		fmt.Fprintf(stderr, "Error: %s\n", msg)
		return false
	}
	return true
}

// parseFlags splits args into positional tokens and --flags. A --name token
// consumes the following token as its value unless that token is itself a
// flag (or absent), in which case --name is recorded as a boolean flag.
func parseFlags(args []string) (positional []string, flags map[string]string, boolFlags map[string]bool) {
	flags = map[string]string{}
	boolFlags = map[string]bool{}
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			name := strings.TrimPrefix(a, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[name] = args[i+1]
				i += 2
			} else {
				boolFlags[name] = true
				i++
			}
		} else {
			positional = append(positional, a)
			i++
		}
	}
	return positional, flags, boolFlags
}
