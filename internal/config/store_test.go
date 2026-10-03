package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSettingsDefaults(t *testing.T) {
	s := OpenAt(t.TempDir())
	cfg, err := s.Settings()
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if cfg.DiffCommand != "git diff" {
		t.Errorf("DiffCommand = %q, want %q", cfg.DiffCommand, "git diff")
	}
	if cfg.Prefix != "ctrl+space" {
		t.Errorf("Prefix = %q, want %q", cfg.Prefix, "ctrl+space")
	}
	if cfg.WorktreeBaseDir == "" {
		t.Error("WorktreeBaseDir is empty, want a default")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s := OpenAt(t.TempDir())
	in := Settings{
		WorktreeBaseDir:           "/tmp/wt",
		DiffCommand:               "git diff --stat",
		PostWorktreeCreateCommand: "true",
		Theme:                     "dark",
		Prefix:                    "ctrl+b",
	}
	if err := s.SaveSettings(in); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	out, err := s.Settings()
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestReposAddRemoveFind(t *testing.T) {
	s := OpenAt(t.TempDir())

	r := NewRepoRecord("/tmp/repo", "repo", "run: claude")
	if r.ID == "" {
		t.Fatal("NewRepoRecord generated no ID")
	}
	if !r.PullBeforeWorktree {
		t.Fatal("NewRepoRecord should default PullBeforeWorktree to true")
	}
	if err := s.AddRepo(r); err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if err := s.AddRepo(RepoRecord{Path: "/tmp/other", Name: "other"}); err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	repos, err := s.Repos()
	if err != nil {
		t.Fatalf("Repos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}
	if repos[1].ID == "" {
		t.Error("AddRepo did not generate an ID for a blank record")
	}

	got, ok, err := s.FindRepo(r.ID)
	if err != nil || !ok {
		t.Fatalf("FindRepo: ok=%v err=%v", ok, err)
	}
	if got.Name != "repo" {
		t.Errorf("FindRepo Name = %q, want %q", got.Name, "repo")
	}

	got.LayoutText = "run:"
	if err := s.UpdateRepo(got); err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}
	got2, _, _ := s.FindRepo(r.ID)
	if got2.LayoutText != "run:" {
		t.Errorf("UpdateRepo not persisted: %q", got2.LayoutText)
	}

	if err := s.RemoveRepo(r.ID); err != nil {
		t.Fatalf("RemoveRepo: %v", err)
	}
	if _, ok, _ := s.FindRepo(r.ID); ok {
		t.Error("RemoveRepo left the record behind")
	}
}

func TestSessionsUpsertAndUpdate(t *testing.T) {
	s := OpenAt(t.TempDir())

	rec := SessionRecord{WorktreePath: "/tmp/wt/a", RepoName: "repo", SessionName: "a", Attached: true}
	if err := s.UpsertSession(rec); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	rec.SessionName = "renamed"
	if err := s.UpsertSession(rec); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	sessions, err := s.Sessions()
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1 (upsert should replace)", len(sessions))
	}
	if sessions[0].SessionName != "renamed" {
		t.Errorf("SessionName = %q, want %q", sessions[0].SessionName, "renamed")
	}

	if err := s.SetSessionAttached("/tmp/wt/a", false); err != nil {
		t.Fatalf("SetSessionAttached: %v", err)
	}
	if err := s.SetClaudeSessionID("/tmp/wt/a", "abc123"); err != nil {
		t.Fatalf("SetClaudeSessionID: %v", err)
	}
	if err := s.SetSessionPR("/tmp/wt/a", &PRInfo{Number: 7, State: "OPEN"}); err != nil {
		t.Fatalf("SetSessionPR: %v", err)
	}
	sessions, _ = s.Sessions()
	got := sessions[0]
	if got.Attached {
		t.Error("Attached = true, want false")
	}
	if got.ClaudeSessionID != "abc123" {
		t.Errorf("ClaudeSessionID = %q, want %q", got.ClaudeSessionID, "abc123")
	}
	if got.PR == nil || got.PR.Number != 7 {
		t.Errorf("PR = %+v, want number 7", got.PR)
	}

	if err := s.RemoveSession("/tmp/wt/a"); err != nil {
		t.Fatalf("RemoveSession: %v", err)
	}
	if sessions, _ := s.Sessions(); len(sessions) != 0 {
		t.Errorf("len(sessions) = %d after remove, want 0", len(sessions))
	}
}

func TestAtomicWriteLeavesCleanDirectory(t *testing.T) {
	dir := t.TempDir()
	s := OpenAt(dir)
	if err := s.SaveSettings(Settings{DiffCommand: "git diff"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "settings.json" && strings.Contains(e.Name(), "settings.json") {
			t.Errorf("stray temp file left behind: %s", e.Name())
		}
	}
}

func TestMissingFilesLoadAsZeroValues(t *testing.T) {
	s := OpenAt(t.TempDir())
	if repos, err := s.Repos(); err != nil || len(repos) != 0 {
		t.Errorf("Repos on empty dir = %v, %v", repos, err)
	}
	if sessions, err := s.Sessions(); err != nil || len(sessions) != 0 {
		t.Errorf("Sessions on empty dir = %v, %v", sessions, err)
	}
}

func TestWriteJSONFollowsSymlink(t *testing.T) {
	s := OpenAt(t.TempDir())
	real := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(real, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seeding real file: %v", err)
	}
	link := filepath.Join(s.Dir, "settings.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := s.SaveSettings(Settings{DiffCommand: "delta"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("settings.json is no longer a symlink")
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatalf("reading real file: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parsing real file: %v", err)
	}
	if got["diffCommand"] != "delta" {
		t.Errorf("diffCommand = %v, want delta", got["diffCommand"])
	}
}
