package cli

import (
	"bytes"
	"testing"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func TestNotifyDefaultColor(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"notify"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionSessionNotify {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionSessionNotify)
	}
	if got.Command != protocol.ColorGreen {
		t.Errorf("command = %q, want %q", got.Command, protocol.ColorGreen)
	}
}

func TestNotifyExplicitColor(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"notify", "red"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Command != "red" {
		t.Errorf("command = %q, want %q", got.Command, "red")
	}
}
