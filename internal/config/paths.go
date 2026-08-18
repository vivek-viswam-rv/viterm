// Package config manages viterm's on-disk configuration: user settings, the
// list of registered repositories, and the record of active worktree
// sessions. Each is stored as a JSON file in a per-user config directory and
// accessed through a Store.
package config

import (
	"os"
	"path/filepath"
)

// configDirName is the name of viterm's directory under the resolved config
// root (either $XDG_CONFIG_HOME or ~/.config).
const configDirName = "viterm"

// Dir returns the directory viterm stores its configuration in:
// $XDG_CONFIG_HOME/viterm if XDG_CONFIG_HOME is set and non-empty, otherwise
// ~/.config/viterm. It does not create the directory; call EnsureDir for
// that.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, configDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", configDirName), nil
}

// EnsureDir creates dir (and any missing parents) with permissions 0755 if
// it does not already exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
