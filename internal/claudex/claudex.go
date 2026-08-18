// Package claudex works with Claude Code's on-disk conventions: the
// per-project transcript folder under ~/.claude/projects, hook event
// payloads, and resume-flag rewriting for run commands.
package claudex

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HomeOverride, when non-empty, replaces ~/.claude as the Claude Code home
// directory. Intended for tests.
var HomeOverride string

func claudeHome() string {
	if HomeOverride != "" {
		return HomeOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".claude")
}

// ProjectSlug converts a worktree path into Claude Code's project folder
// name: the absolute path with every '/' and '.' replaced by '-'.
func ProjectSlug(worktreePath string) string {
	p := worktreePath
	if abs, err := filepath.Abs(worktreePath); err == nil {
		p = abs
	}
	p = strings.ReplaceAll(p, "/", "-")
	return strings.ReplaceAll(p, ".", "-")
}

// ProjectDir returns the transcript folder for a worktree.
func ProjectDir(worktreePath string) string {
	return filepath.Join(claudeHome(), "projects", ProjectSlug(worktreePath))
}

// HasProject reports whether a transcript folder exists for the worktree.
func HasProject(worktreePath string) bool {
	info, err := os.Stat(ProjectDir(worktreePath))
	return err == nil && info.IsDir()
}

// SessionFileExists reports whether a transcript exists for the given
// session ID.
func SessionFileExists(worktreePath, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(ProjectDir(worktreePath), sessionID+".jsonl"))
	return err == nil
}

// NewestSessionID returns the most recently modified transcript's session ID
// for the worktree.
func NewestSessionID(worktreePath string) (string, bool) {
	entries, err := os.ReadDir(ProjectDir(worktreePath))
	if err != nil {
		return "", false
	}
	var best string
	var bestMod int64 = -1
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixNano(); mod > bestMod {
			bestMod = mod
			best = strings.TrimSuffix(e.Name(), ".jsonl")
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// RemoveProject deletes the worktree's transcript folder.
func RemoveProject(worktreePath string) error {
	return os.RemoveAll(ProjectDir(worktreePath))
}

// HookPayload is the subset of a Claude Code hook event payload viterm uses.
type HookPayload struct {
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
}

// ParseHookPayload decodes a hook payload, tolerating unknown fields and
// empty input.
func ParseHookPayload(r io.Reader) (HookPayload, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return HookPayload{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return HookPayload{}, nil
	}
	var p HookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return HookPayload{}, err
	}
	return p, nil
}

// RewriteRunCommand appends a resume flag to a claude invocation when a
// prior conversation exists for the worktree: --resume <id> when the
// recorded session's transcript is present, otherwise --continue when the
// project folder exists. Commands that are not plain claude invocations, or
// that already carry a resume flag, are returned unchanged.
func RewriteRunCommand(cmd, sessionID, worktreePath string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 || fields[0] != "claude" {
		return cmd
	}
	if strings.Contains(cmd, "--continue") || strings.Contains(cmd, "--resume") {
		return cmd
	}
	if sessionID != "" && SessionFileExists(worktreePath, sessionID) {
		return cmd + " --resume " + sessionID
	}
	if HasProject(worktreePath) {
		return cmd + " --continue"
	}
	return cmd
}
