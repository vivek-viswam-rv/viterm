package claudex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	HomeOverride = home
	t.Cleanup(func() { HomeOverride = "" })
	return home
}

func TestProjectSlug(t *testing.T) {
	got := ProjectSlug("/Users/dev/code/my.project")
	if strings.ContainsAny(got, "/.") {
		t.Errorf("ProjectSlug left separators in %q", got)
	}
	if got != "-Users-dev-code-my-project" {
		t.Errorf("ProjectSlug = %q, want %q", got, "-Users-dev-code-my-project")
	}
}

func TestHasProjectAndSessionFile(t *testing.T) {
	withHome(t)
	worktree := "/tmp/wt/example"
	if HasProject(worktree) {
		t.Fatal("HasProject = true before the folder exists")
	}
	dir := ProjectDir(worktree)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !HasProject(worktree) {
		t.Fatal("HasProject = false after creating the folder")
	}
	if SessionFileExists(worktree, "abc") {
		t.Fatal("SessionFileExists = true before the file exists")
	}
	if err := os.WriteFile(filepath.Join(dir, "abc.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !SessionFileExists(worktree, "abc") {
		t.Fatal("SessionFileExists = false for an existing transcript")
	}
	if SessionFileExists(worktree, "") {
		t.Fatal("SessionFileExists = true for an empty ID")
	}
}

func TestNewestSessionID(t *testing.T) {
	withHome(t)
	worktree := "/tmp/wt/example"
	if _, ok := NewestSessionID(worktree); ok {
		t.Fatal("NewestSessionID found something in a missing folder")
	}
	dir := ProjectDir(worktree)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "older.jsonl")
	newer := filepath.Join(dir, "newer.jsonl")
	for _, f := range []string{old, newer} {
		if err := os.WriteFile(f, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Minute), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	id, ok := NewestSessionID(worktree)
	if !ok || id != "newer" {
		t.Errorf("NewestSessionID = %q, %v; want %q, true", id, ok, "newer")
	}
}

func TestRemoveProject(t *testing.T) {
	withHome(t)
	worktree := "/tmp/wt/example"
	if err := os.MkdirAll(ProjectDir(worktree), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveProject(worktree); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	if HasProject(worktree) {
		t.Fatal("project folder still present after RemoveProject")
	}
}

func TestParseHookPayload(t *testing.T) {
	p, err := ParseHookPayload(strings.NewReader(`{"session_id":"s1","cwd":"/tmp","extra":42}`))
	if err != nil {
		t.Fatalf("ParseHookPayload: %v", err)
	}
	if p.SessionID != "s1" || p.CWD != "/tmp" {
		t.Errorf("payload = %+v", p)
	}

	p, err = ParseHookPayload(strings.NewReader("  \n"))
	if err != nil || p != (HookPayload{}) {
		t.Errorf("empty input = %+v, %v; want zero value, nil", p, err)
	}

	if _, err := ParseHookPayload(strings.NewReader("{not json")); err == nil {
		t.Error("invalid JSON did not error")
	}
}

func TestRewriteRunCommand(t *testing.T) {
	withHome(t)
	worktree := "/tmp/wt/example"
	dir := ProjectDir(worktree)

	// No project folder: unchanged.
	if got := RewriteRunCommand("claude", "", worktree); got != "claude" {
		t.Errorf("no project: %q", got)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, cmd, sessionID, want string
	}{
		{"non-claude command", "vim .", "", "vim ."},
		{"claude-prefixed binary", "claude-wrapper go", "", "claude-wrapper go"},
		{"already continues", "claude --continue", "", "claude --continue"},
		{"already resumes", "claude --resume x", "", "claude --resume x"},
		{"project folder only", "claude", "", "claude --continue"},
		{"missing session file falls back", "claude", "nope", "claude --continue"},
	}
	for _, tt := range tests {
		if got := RewriteRunCommand(tt.cmd, tt.sessionID, worktree); got != tt.want {
			t.Errorf("%s: RewriteRunCommand = %q, want %q", tt.name, got, tt.want)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "sess1.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RewriteRunCommand("claude --model x", "sess1", worktree); got != "claude --model x --resume sess1" {
		t.Errorf("resume: %q", got)
	}
}
