package cli

import (
	"bytes"
	"testing"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func TestRunCommandBasic(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)
	t.Setenv(protocol.EnvPaneID, "pane-1")

	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "echo", "hi"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionTerminalRun {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionTerminalRun)
	}
	if got.Command != "echo hi" {
		t.Errorf("command = %q, want %q", got.Command, "echo hi")
	}
	if got.PaneSeq != nil {
		t.Errorf("paneSeq = %v, want nil", got.PaneSeq)
	}
}

func TestRunWithPaneFlag(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "--pane", "2", "echo", "hi"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.PaneSeq == nil || *got.PaneSeq != 2 {
		t.Errorf("paneSeq = %v, want *2", got.PaneSeq)
	}
	if got.Command != "echo hi" {
		t.Errorf("command = %q, want %q", got.Command, "echo hi")
	}
}

func TestRunUsageError(t *testing.T) {
	called := false
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		called = true
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"run"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if called {
		t.Errorf("handler should not have been called for a usage error")
	}
}

func TestRunInvalidPaneValue(t *testing.T) {
	socket := startFakeServer(t, func(cmd protocol.Command) []byte { return nil })
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "--pane", "not-a-number", "echo", "hi"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
