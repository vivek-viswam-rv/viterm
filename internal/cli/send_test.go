package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/vivekviswam/viterm/internal/protocol"
)

func TestSendCommand(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		b, _ := json.Marshal(protocol.Reply{OK: true})
		return b
	})
	t.Setenv(protocol.EnvSocket, socket)
	t.Setenv(protocol.EnvPaneID, "pane-9")

	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "3", "hello", "world"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionTabSend {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionTabSend)
	}
	if got.TabID != "3" {
		t.Errorf("tabId = %q, want %q", got.TabID, "3")
	}
	if got.Text != "hello world" {
		t.Errorf("text = %q, want %q", got.Text, "hello world")
	}
	if got.PaneID != "pane-9" {
		t.Errorf("paneId = %q, want %q", got.PaneID, "pane-9")
	}
	if stdout.String() != "Sent to 3\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestSendUnescape(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		b, _ := json.Marshal(protocol.Reply{OK: true})
		return b
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "1", `line1\nline2\tend`}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	want := "line1\nline2\tend"
	if got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
}

func TestSendFailureReply(t *testing.T) {
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		b, _ := json.Marshal(protocol.Reply{OK: false, Error: "no such tab"})
		return b
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "99", "hi"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("no such tab")) {
		t.Errorf("stderr = %q, want it to contain the server error", stderr.String())
	}
}

func TestSendUsageError(t *testing.T) {
	called := false
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		called = true
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "1"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if called {
		t.Errorf("handler should not have been called for a usage error")
	}
}

func TestUnescapeTextFunc(t *testing.T) {
	cases := []struct{ in, want string }{
		{"no escapes here", "no escapes here"},
		{`a\nb`, "a\nb"},
		{`a\tb`, "a\tb"},
		{`a\nb\tc`, "a\nb\tc"},
		{"already\nreal\nnewline", "already\nreal\nnewline"},
		{`mix \n and \t and more \n`, "mix \n and \t and more \n"},
	}
	for _, c := range cases {
		if got := unescapeText(c.in); got != c.want {
			t.Errorf("unescapeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
