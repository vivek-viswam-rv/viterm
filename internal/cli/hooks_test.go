package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func settingsPathFor(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return b
}

func decodeSettings(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decoding settings JSON: %v (raw: %s)", err, b)
	}
	return m
}

// hookCommands collects every hook "command" string under settings["hooks"][event].
func hookCommands(m map[string]any, event string) []string {
	hooks, _ := m["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	var out []string
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		hookList, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hookList {
			entry, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := entry["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func containsSubstring(list []string, substr string) bool {
	for _, s := range list {
		if bytes.Contains([]byte(s), []byte(substr)) {
			return true
		}
	}
	return false
}

func TestInstallHooksFresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	code := run([]string{"install-hooks"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	path := settingsPathFor(home)
	b := mustReadFile(t, path)
	m := decodeSettings(t, b)

	for _, event := range []string{"Stop", "Notification", "UserPromptSubmit"} {
		cmds := hookCommands(m, event)
		if len(cmds) != 1 || !containsSubstring(cmds, "VITERM_SOCKET") {
			t.Errorf("event %s: commands = %v, want exactly one containing VITERM_SOCKET", event, cmds)
		}
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("expected no .bak file for a fresh install, stat err = %v", err)
	}
}

func TestInstallHooksPreservesUnrelated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := settingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := []byte(`{
  "foo": "bar",
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {"type": "command", "command": "echo unrelated"}
        ]
      }
    ]
  }
}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seeding settings.json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"install-hooks"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	m := decodeSettings(t, mustReadFile(t, path))
	if m["foo"] != "bar" {
		t.Errorf("unrelated top-level key foo = %v, want %q", m["foo"], "bar")
	}
	preToolUse := hookCommands(m, "PreToolUse")
	if len(preToolUse) != 1 || preToolUse[0] != "echo unrelated" {
		t.Errorf("PreToolUse commands = %v, want unchanged", preToolUse)
	}
	for _, event := range []string{"Stop", "Notification", "UserPromptSubmit"} {
		cmds := hookCommands(m, event)
		if len(cmds) != 1 || !containsSubstring(cmds, "VITERM_SOCKET") {
			t.Errorf("event %s: commands = %v, want exactly one containing VITERM_SOCKET", event, cmds)
		}
	}

	bak := mustReadFile(t, path+".bak")
	if !bytes.Equal(bak, original) {
		t.Errorf(".bak content mismatch:\ngot:\n%s\nwant:\n%s", bak, original)
	}
}

func TestInstallHooksAlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := settingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := []byte(`{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "[ -n \"$VITERM_SOCKET\" ] && viterm claude-event stop || true"}]}
    ],
    "Notification": [
      {"matcher": "permission_prompt", "hooks": [{"type": "command", "command": "[ -n \"$VITERM_SOCKET\" ] && viterm claude-event permission || true"}]}
    ],
    "UserPromptSubmit": [
      {"hooks": [{"type": "command", "command": "[ -n \"$VITERM_SOCKET\" ] && viterm claude-event prompt-submit || true"}]}
    ]
  }
}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seeding settings.json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"install-hooks"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	after := mustReadFile(t, path)
	if !bytes.Equal(after, original) {
		t.Errorf("settings.json changed when hooks were already installed:\nbefore:\n%s\nafter:\n%s", original, after)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("expected no .bak churn when nothing changed, stat err = %v", err)
	}
}

func TestInstallHooksPartiallyInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := settingsPathFor(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := []byte(`{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "[ -n \"$VITERM_SOCKET\" ] && viterm claude-event stop || true"}]}
    ]
  }
}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seeding settings.json: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"install-hooks"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	m := decodeSettings(t, mustReadFile(t, path))
	stopCmds := hookCommands(m, "Stop")
	if len(stopCmds) != 1 {
		t.Errorf("Stop commands = %v, want exactly the original single entry (not duplicated)", stopCmds)
	}
	for _, event := range []string{"Notification", "UserPromptSubmit"} {
		cmds := hookCommands(m, event)
		if len(cmds) != 1 || !containsSubstring(cmds, "VITERM_SOCKET") {
			t.Errorf("event %s: commands = %v, want exactly one containing VITERM_SOCKET", event, cmds)
		}
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("expected a .bak file since the file did change, stat err = %v", err)
	}
}

func TestInstallHooksDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout, stderr bytes.Buffer
	code := run([]string{"install-hooks", "--dry-run"}, "test", nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	if !bytes.Contains(stdout.Bytes(), []byte("VITERM_SOCKET")) {
		t.Errorf("dry-run stdout should contain the would-be hook JSON, got: %s", stdout.String())
	}

	path := settingsPathFor(home)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("dry-run must not write any file, stat err = %v", err)
	}
}
