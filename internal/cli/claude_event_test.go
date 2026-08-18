package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func TestClaudeEventStdinParsing(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)
	t.Setenv(protocol.EnvPaneID, "pane-7")

	stdin := strings.NewReader(`{"session_id":"abc123","cwd":"/tmp/project"}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude-event", "stop"}, "test", stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionClaudeEvent {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionClaudeEvent)
	}
	if got.Command != protocol.ClaudeEventStop {
		t.Errorf("command = %q, want %q", got.Command, protocol.ClaudeEventStop)
	}
	if got.ClaudeSessionID != "abc123" {
		t.Errorf("claudeSessionId = %q, want %q", got.ClaudeSessionID, "abc123")
	}
	if got.CWD != "/tmp/project" {
		t.Errorf("cwd = %q, want %q", got.CWD, "/tmp/project")
	}
	if got.PaneID != "pane-7" {
		t.Errorf("paneId = %q, want %q", got.PaneID, "pane-7")
	}
}

func TestClaudeEventEmptyStdin(t *testing.T) {
	var got protocol.Command
	called := false
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		called = true
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude-event", "permission"}, "test", stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
	if got.ClaudeSessionID != "" || got.CWD != "" {
		t.Errorf("expected zero-valued session/cwd for empty stdin, got %+v", got)
	}
	if got.Command != protocol.ClaudeEventPermission {
		t.Errorf("command = %q, want %q", got.Command, protocol.ClaudeEventPermission)
	}
}

func TestClaudeEventMalformedStdin(t *testing.T) {
	called := false
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		called = true
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	stdin := strings.NewReader("{not json at all")
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude-event", "prompt-submit"}, "test", stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !called {
		t.Fatalf("expected handler to still be called despite malformed stdin JSON")
	}
}

func TestClaudeEventNoSocketIsSilent(t *testing.T) {
	orig, hadOrig := os.LookupEnv(protocol.EnvSocket)
	os.Unsetenv(protocol.EnvSocket)
	t.Cleanup(func() {
		if hadOrig {
			os.Setenv(protocol.EnvSocket, orig)
		}
	})

	stdin := strings.NewReader(`{"session_id":"x"}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude-event", "stop"}, "test", stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent no-op, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestClaudeEventSocketGoneIsSilent(t *testing.T) {
	// Point at a socket path that doesn't exist (nothing is listening).
	t.Setenv(protocol.EnvSocket, "/tmp/viterm-does-not-exist-12345.sock")

	stdin := strings.NewReader(`{}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude-event", "stop"}, "test", stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent no-op, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
