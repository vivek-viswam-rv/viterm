package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/vivek-viswam-rv/viterm/internal/ipc"
	"github.com/vivek-viswam-rv/viterm/internal/protocol"
)

func startFakeServer(t *testing.T, handler ipc.Handler) string {
	t.Helper()
	srv, err := ipc.NewServer()
	if err != nil {
		t.Fatalf("ipc.NewServer: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	if err := srv.Start(handler); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return srv.Path()
}

func TestTabsTableGolden(t *testing.T) {
	canned := []protocol.TabListEntry{
		{TabID: "t2", TabSeq: 2, PaneID: "p1", PaneSeq: 1, Type: "link", Title: "docs", IsActive: false},
		{TabID: "t1", TabSeq: 1, PaneID: "p1", PaneSeq: 1, Type: "terminal", Title: "zsh", IsActive: true},
	}
	body, err := json.Marshal(canned)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		if cmd.Action != protocol.ActionTabsList {
			t.Errorf("unexpected action: %s", cmd.Action)
		}
		return body
	})
	t.Setenv(protocol.EnvSocket, socket)
	t.Setenv(protocol.EnvPaneID, "p1")

	var stdout, stderr bytes.Buffer
	code := run([]string{"tabs"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	want := "TAB  PANE  TYPE      TITLE\n" +
		"--------------------------------------------------\n" +
		"1    1     terminal  zsh *\n" +
		"2    1     link      docs\n"
	if stdout.String() != want {
		t.Fatalf("stdout mismatch:\ngot:\n%q\nwant:\n%q", stdout.String(), want)
	}
}

func TestTabsEmpty(t *testing.T) {
	socket := startFakeServer(t, func(cmd protocol.Command) []byte {
		b, _ := json.Marshal([]protocol.TabListEntry{})
		return b
	})
	t.Setenv(protocol.EnvSocket, socket)

	var stdout, stderr bytes.Buffer
	code := run([]string{"tabs"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "No tabs.\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "No tabs.\n")
	}
}

func TestTabsNoSocket(t *testing.T) {
	orig, hadOrig := os.LookupEnv(protocol.EnvSocket)
	os.Unsetenv(protocol.EnvSocket)
	t.Cleanup(func() {
		if hadOrig {
			os.Setenv(protocol.EnvSocket, orig)
		}
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"tabs"}, "test", nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr.String() == "" {
		t.Fatalf("expected an error message on stderr")
	}
}
