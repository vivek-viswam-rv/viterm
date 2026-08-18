// Package protocol defines the wire schema shared by the viterm TUI (server)
// and the viterm CLI (client). Commands travel as a single JSON object over a
// per-session Unix domain socket: the client writes one Command, half-closes
// the write side, and reads an optional reply until EOF.
package protocol

// Actions understood by a running viterm session.
const (
	ActionTabsList      = "tabs.list"
	ActionTabSend       = "tab.send"
	ActionTerminalRun   = "terminal.run"
	ActionURLOpen       = "url.open"
	ActionSessionNotify = "session.notify"
	ActionClaudeEvent   = "claude.event"
)

// Environment variables injected into every pane's shell.
const (
	EnvSocket = "VITERM_SOCKET"
	EnvPaneID = "VITERM_PANE_ID"
)

// Command is the single request object exchanged over the socket. All fields
// except Action are optional and their meaning depends on the action.
type Command struct {
	Action string `json:"action"`

	// PaneID is the UUID of the pane the client is running in, taken from
	// VITERM_PANE_ID. Used as the default routing target.
	PaneID string `json:"paneId,omitempty"`
	// PaneSeq, when set, overrides PaneID as the routing target. Panes are
	// numbered from 1 in creation order for the current run.
	PaneSeq *int `json:"paneSeq,omitempty"`

	// TabID identifies a tab for tab.send. It may be a tab UUID or the short
	// sequence number printed by `viterm tabs`.
	TabID string `json:"tabId,omitempty"`
	// Text is the payload for tab.send.
	Text string `json:"text,omitempty"`

	// Command carries the shell command for terminal.run, the color name for
	// session.notify, or the event kind for claude.event.
	Command string `json:"command,omitempty"`

	// URL is the target for url.open.
	URL string `json:"url,omitempty"`
	// Background, for url.open, adds the link tab without focusing it.
	Background *bool `json:"background,omitempty"`

	// ClaudeSessionID and CWD accompany claude.event and are read from the
	// hook payload delivered on stdin.
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
	CWD             string `json:"cwd,omitempty"`
}

// Reply is the generic acknowledgement for actions that answer.
type Reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// TabListEntry describes one tab in a tabs.list response. The response body is
// a JSON array of entries.
type TabListEntry struct {
	TabID    string `json:"tabId"`
	TabSeq   int    `json:"tabSeq"`
	PaneID   string `json:"paneId"`
	PaneSeq  int    `json:"paneSeq"`
	Type     string `json:"type"` // "terminal" or "link"
	Title    string `json:"title"`
	IsActive bool   `json:"isActive"`
}

// Notify color names accepted by session.notify. Any unknown non-empty value
// is treated as green; ColorClear (and aliases "none", "reset") clears the
// session tint.
const (
	ColorGreen  = "green"
	ColorRed    = "red"
	ColorYellow = "yellow"
	ColorBlue   = "blue"
	ColorOrange = "orange"
	ColorClear  = "clear"
)

// Claude hook event kinds carried by claude.event.
const (
	ClaudeEventStop         = "stop"
	ClaudeEventPermission   = "permission"
	ClaudeEventPromptSubmit = "prompt-submit"
)
