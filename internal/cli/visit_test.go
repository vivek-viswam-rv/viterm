package cli

import (
	"bytes"
	"testing"

	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func TestVisitBasic(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"visit", "https://example.com"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionURLOpen {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionURLOpen)
	}
	if got.URL != "https://example.com" {
		t.Errorf("url = %q", got.URL)
	}
	if got.PaneSeq != nil {
		t.Errorf("paneSeq = %v, want nil", got.PaneSeq)
	}
	if got.Background != nil {
		t.Errorf("background = %v, want nil", got.Background)
	}
}

func TestVisitFlags(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"visit", "https://example.com", "--pane", "3", "--background"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.PaneSeq == nil || *got.PaneSeq != 3 {
		t.Errorf("paneSeq = %v, want *3", got.PaneSeq)
	}
	if got.Background == nil || !*got.Background {
		t.Errorf("background = %v, want *true", got.Background)
	}
	if got.URL != "https://example.com" {
		t.Errorf("url = %q", got.URL)
	}
}

func TestBrowserOpenAlias(t *testing.T) {
	var got protocol.Command
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		got = cmd
		return nil
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"browser", "open", "https://example.com", "--pane", "1"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got.Action != protocol.ActionURLOpen {
		t.Errorf("action = %q, want %q", got.Action, protocol.ActionURLOpen)
	}
	if got.URL != "https://example.com" {
		t.Errorf("url = %q", got.URL)
	}
	if got.PaneSeq == nil || *got.PaneSeq != 1 {
		t.Errorf("paneSeq = %v, want *1", got.PaneSeq)
	}
}

func TestBrowserBadSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"browser", "close", "https://example.com"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestVisitUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"visit"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
