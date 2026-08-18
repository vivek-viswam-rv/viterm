package ipc

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

// Send dials the Unix domain socket at socketPath, writes cmd as a single
// JSON object, half-closes the write side so the server can observe EOF, and
// reads the reply (if any) until the server closes the connection. It
// returns the raw reply bytes, which may be empty for actions that produce
// no reply.
func Send(socketPath string, cmd protocol.Command) ([]byte, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("ipc: dialing %s: %w", socketPath, err)
	}
	defer conn.Close()

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("ipc: marshaling command: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("ipc: writing request: %w", err)
	}

	if uc, ok := conn.(*net.UnixConn); ok {
		if err := uc.CloseWrite(); err != nil {
			return nil, fmt.Errorf("ipc: closing write side: %w", err)
		}
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	reply, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("ipc: reading reply: %w", err)
	}
	return reply, nil
}
