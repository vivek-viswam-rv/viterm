package config

import (
	"os"
	"path/filepath"
)

const settingsFile = "settings.json"

// Settings holds user-tunable behavior. Zero-valued fields are filled with
// defaults when loaded.
type Settings struct {
	// WorktreeBaseDir is the root under which session worktrees are created.
	WorktreeBaseDir string `json:"worktreeBaseDir"`
	// DiffCommand runs inside a maximized terminal tab for the diff view.
	DiffCommand string `json:"diffCommand"`
	// PostWorktreeCreateCommand runs through the login shell after a
	// worktree is created, with WORKTREE_DIRECTORY set.
	PostWorktreeCreateCommand string `json:"postWorktreeCreateCommand"`
	// Theme names the color theme. An empty value keeps the adaptive
	// default: the app chrome uses the hosting terminal's ANSI palette,
	// and the windowed host uses the built-in default theme.
	Theme string `json:"theme,omitempty"`
	// Colors overrides individual chrome colors with hex values. Recognized
	// keys: accent, dim, foreground, stripBackground.
	Colors map[string]string `json:"colors,omitempty"`
	// Font and FontSize style the windowed app host's terminal surface.
	Font     string `json:"font,omitempty"`
	FontSize int    `json:"fontSize,omitempty"`
	// Prefix is the key that introduces viterm commands, in a form like
	// "ctrl+space" or "ctrl+b".
	Prefix string `json:"prefix,omitempty"`
}

// Settings loads settings.json, applying defaults to empty fields.
func (s *Store) Settings() (Settings, error) {
	var cfg Settings
	if err := s.readJSON(settingsFile, &cfg); err != nil {
		return cfg, err
	}
	if cfg.WorktreeBaseDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.WorktreeBaseDir = filepath.Join(home, "code", "viterm-worktrees")
		}
	}
	if cfg.DiffCommand == "" {
		cfg.DiffCommand = "git diff"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "ctrl+space"
	}
	return cfg, nil
}

// SaveSettings persists settings.json.
func (s *Store) SaveSettings(cfg Settings) error {
	return s.writeJSON(settingsFile, cfg)
}
