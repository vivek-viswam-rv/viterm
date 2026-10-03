package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// vitermMarker is the substring install-hooks looks for in an event's
// existing hook commands to decide whether that event is already wired up.
// It is deliberately the literal env var name, so any pre-existing hook that
// already references VITERM_SOCKET (installed by an earlier run of this
// same binary) is recognized and left untouched.
const vitermMarker = "VITERM_SOCKET"

type hookEventSpec struct {
	Event   string
	Matcher string // empty means a matcher-less hook group
	Command string
}

var vitermHookSpecs = []hookEventSpec{
	{
		Event:   "Stop",
		Command: `[ -n "$VITERM_SOCKET" ] && viterm claude-event stop || true`,
	},
	{
		Event:   "Notification",
		Matcher: "permission_prompt",
		Command: `[ -n "$VITERM_SOCKET" ] && viterm claude-event permission || true`,
	},
	{
		Event:   "UserPromptSubmit",
		Command: `[ -n "$VITERM_SOCKET" ] && viterm claude-event prompt-submit || true`,
	},
}

// installHooks idempotently merges viterm's Claude Code hooks into
// ~/.claude/settings.json, one event at a time: an event whose existing
// hook commands already mention VITERM_SOCKET is left completely untouched,
// while missing events get viterm's hook group appended. It never touches
// the socket.
func installHooks(dryRun bool, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not determine home directory: %v\n", err)
		return 1
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	// Dotfile setups often symlink settings.json into a repository; writing
	// through the link keeps that arrangement intact.
	if resolved, err := filepath.EvalSymlinks(settingsPath); err == nil {
		settingsPath = resolved
	}

	origBytes, readErr := os.ReadFile(settingsPath)
	existed := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		fmt.Fprintf(stderr, "Error: reading %s: %v\n", settingsPath, readErr)
		return 1
	}

	settings := map[string]any{}
	if existed {
		if err := json.Unmarshal(origBytes, &settings); err != nil {
			fmt.Fprintf(stderr, "Error: %s contains invalid JSON: %v\n", settingsPath, err)
			return 1
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	changed := false
	for _, spec := range vitermHookSpecs {
		groups, _ := hooks[spec.Event].([]any)
		if hookGroupsInstalled(groups) {
			continue
		}

		group := map[string]any{
			"hooks": []any{
				map[string]any{"type": "command", "command": spec.Command},
			},
		}
		if spec.Matcher != "" {
			group["matcher"] = spec.Matcher
		}
		hooks[spec.Event] = append(groups, group)
		changed = true
	}
	settings["hooks"] = hooks

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "Error: encoding settings: %v\n", err)
		return 1
	}
	output = append(output, '\n')

	if dryRun {
		_, _ = stdout.Write(output)
		return 0
	}

	if !changed {
		fmt.Fprintf(stdout, "viterm hooks are already installed in %s\n", settingsPath)
		return 0
	}

	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "Error: creating %s: %v\n", dir, err)
		return 1
	}

	if existed {
		if err := os.WriteFile(settingsPath+".bak", origBytes, 0o644); err != nil {
			fmt.Fprintf(stderr, "Error: writing backup %s.bak: %v\n", settingsPath, err)
			return 1
		}
	}

	if err := writeFileAtomic(settingsPath, output, 0o644); err != nil {
		fmt.Fprintf(stderr, "Error: writing %s: %v\n", settingsPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "Installed viterm hooks in %s\n", settingsPath)
	return 0
}

// hookGroupsInstalled reports whether any hook command in groups already
// references VITERM_SOCKET, i.e. whether this event has already been wired
// up (by this or an equivalent earlier install-hooks run).
func hookGroupsInstalled(groups []any) bool {
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
			command, _ := entry["command"].(string)
			if strings.Contains(command, vitermMarker) {
				return true
			}
		}
	}
	return false
}

// writeFileAtomic writes data to path by writing to a temp file in the same
// directory and renaming it into place, so a crash or concurrent reader
// never observes a partially-written settings file.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".viterm-settings-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
