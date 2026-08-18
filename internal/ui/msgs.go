package ui

import (
	"github.com/vivek-viswam-rv/viterm/internal/ghx"
	"github.com/vivek-viswam-rv/viterm/internal/protocol"
	"github.com/vivek-viswam-rv/viterm/internal/term"
)

// sockCmdMsg carries one socket request into the update loop. The reply
// channel receives the raw reply bytes (or nil for no reply).
type sockCmdMsg struct {
	session *Session
	cmd     protocol.Command
	reply   chan []byte
}

// paneEventMsg carries a terminal pane event into the update loop.
type paneEventMsg struct {
	session *Session
	tabID   string
	event   term.Event
}

// gitStatusMsg reports refreshed git status for a worktree.
type gitStatusMsg struct {
	worktree string
	branch   string
	sha      string
	add, del int
}

// prStatusMsg reports a resolved pull request for a worktree.
type prStatusMsg struct {
	worktree string
	pr       ghx.PR
	found    bool
}

// gitTickMsg and prTickMsg schedule the status pollers.
type gitTickMsg struct{}
type prTickMsg struct{}

// sessionCreatedMsg reports the outcome of an asynchronous session creation.
type sessionCreatedMsg struct {
	name         string
	repoName     string
	worktreePath string
	slug         string
	layoutText   string
	err          error
}
