package ipc

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func TestRoundTrip(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	var gotCmd protocol.Command
	if err := srv.Start(func(cmd protocol.Command) []byte {
		gotCmd = cmd
		b, _ := json.Marshal(protocol.Reply{OK: true})
		return b
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cmd := protocol.Command{Action: protocol.ActionTabsList, PaneID: "pane-1"}
	reply, err := Send(srv.Path(), cmd)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var r protocol.Reply
	if err := json.Unmarshal(reply, &r); err != nil {
		t.Fatalf("unmarshal reply: %v (raw: %s)", err, reply)
	}
	if !r.OK {
		t.Fatalf("expected OK reply, got %+v", r)
	}
	if gotCmd.Action != protocol.ActionTabsList || gotCmd.PaneID != "pane-1" {
		t.Fatalf("handler received unexpected command: %+v", gotCmd)
	}
}

func TestMalformedJSON(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	if err := srv.Start(func(cmd protocol.Command) []byte {
		t.Errorf("handler should not be called for malformed input")
		return nil
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	raw, err := dialAndRoundTrip(srv.Path(), []byte("{not valid json"))
	if err != nil {
		t.Fatalf("dialAndRoundTrip: %v", err)
	}

	var r protocol.Reply
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("unmarshal reply: %v (raw: %s)", err, raw)
	}
	if r.OK {
		t.Fatalf("expected OK=false for malformed JSON, got %+v", r)
	}
	if r.Error == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestHandlerPanic(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	if err := srv.Start(func(cmd protocol.Command) []byte {
		panic("boom")
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	reply, err := Send(srv.Path(), protocol.Command{Action: "whatever"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var r protocol.Reply
	if err := json.Unmarshal(reply, &r); err != nil {
		t.Fatalf("unmarshal reply: %v (raw: %s)", err, reply)
	}
	if r.OK {
		t.Fatalf("expected OK=false after handler panic, got %+v", r)
	}
	if !strings.Contains(r.Error, "boom") {
		t.Fatalf("expected panic message in error, got %q", r.Error)
	}
}

func TestNoReply(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	if err := srv.Start(func(cmd protocol.Command) []byte {
		return nil
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	reply, err := Send(srv.Path(), protocol.Command{Action: "fire-and-forget"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(reply) != 0 {
		t.Fatalf("expected empty reply, got %q", reply)
	}
}

func TestCleanStale(t *testing.T) {
	dir := os.TempDir()

	// A dead pid: spawn a trivial child process and wait for it to exit, so
	// its pid is guaranteed not to be alive (and very unlikely to have been
	// recycled by the time this test runs).
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running child process: %v", err)
	}
	deadPID := cmd.Process.Pid

	deadPath := filepath.Join(dir, "viterm-"+strconv.Itoa(deadPID)+"-"+hex.EncodeToString([]byte{0xaa, 0xaa, 0xaa, 0xaa})+".sock")
	livePath := filepath.Join(dir, "viterm-"+strconv.Itoa(os.Getpid())+"-"+hex.EncodeToString([]byte{0xbb, 0xbb, 0xbb, 0xbb})+".sock")

	for _, p := range []string{deadPath, livePath} {
		if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
			t.Fatalf("creating stale-socket fixture %s: %v", p, err)
		}
	}
	defer os.Remove(livePath)

	if err := CleanStale(); err != nil {
		t.Fatalf("CleanStale: %v", err)
	}

	if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
		t.Fatalf("expected dead-pid socket file to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(livePath); err != nil {
		t.Fatalf("expected live-pid socket file to be preserved, stat err = %v", err)
	}
}

// dialAndRoundTrip performs the same protocol as Send but writes raw bytes
// instead of a marshaled Command, so tests can exercise malformed input.
func dialAndRoundTrip(socketPath string, payload []byte) ([]byte, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		return nil, err
	}
	if uc, ok := conn.(*net.UnixConn); ok {
		if err := uc.CloseWrite(); err != nil {
			return nil, err
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return io.ReadAll(conn)
}
