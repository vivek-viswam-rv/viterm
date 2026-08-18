package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vivekviswam/viterm/internal/claudex"
	"github.com/vivekviswam/viterm/internal/config"
	"github.com/vivekviswam/viterm/internal/gitx"
	"github.com/vivekviswam/viterm/internal/ipc"
	"github.com/vivekviswam/viterm/internal/layout"
	"github.com/vivekviswam/viterm/internal/protocol"
	"github.com/vivekviswam/viterm/internal/shellx"
	"github.com/vivekviswam/viterm/internal/term"
)

// Session is one open workspace: a worktree, its socket server, and a split
// tree of panes.
type Session struct {
	Record config.SessionRecord

	Tree      *TreeNode
	FocusedID string // pane ID
	Maximized string // pane ID, empty when none

	// Tint is the notification color name for the session tab, empty when
	// clear.
	Tint string

	// Status shown in the footer.
	Branch     string
	ShortSHA   string
	DiffAdd    int
	DiffDel    int
	PR         *config.PRInfo
	DiffTabID  string // tab opened by the diff view
	DiffPaneID string

	server *ipc.Server
	app    *App
}

// SocketPath returns the session's socket path, or empty before startup.
func (s *Session) SocketPath() string {
	if s.server == nil {
		return ""
	}
	return s.server.Path()
}

// startServer creates the session socket and routes commands into the app's
// update loop.
func (s *Session) startServer(app *App) error {
	server, err := ipc.NewServer()
	if err != nil {
		return err
	}
	s.server = server
	s.app = app
	return server.Start(func(cmd protocol.Command) []byte {
		reply := make(chan []byte, 1)
		app.send(sockCmdMsg{session: s, cmd: cmd, reply: reply})
		select {
		case b := <-reply:
			return b
		case <-time.After(5 * time.Second):
			return []byte(`{"ok":false,"error":"session did not respond"}`)
		}
	})
}

// paneEnv builds the env entries injected into a pane's shell.
func (s *Session) paneEnv(paneID string) []string {
	env := []string{
		protocol.EnvSocket + "=" + s.SocketPath(),
		protocol.EnvPaneID + "=" + paneID,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	}
	if dir := executableDir(); dir != "" {
		env = append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return env
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// newTerminalTab starts a terminal tab in the session's worktree. command,
// when non-empty, is typed into the shell shortly after startup.
func (s *Session) newTerminalTab(p *Pane, command string, cols, rows int) (*Tab, error) {
	tab := &Tab{ID: newID(), Seq: s.app.tabSeq.next(), Command: command}
	notify := s.app.notifyFunc(s, tab.ID)
	tp, err := term.Start(term.Options{
		Shell:  shellx.LoginShell(),
		Dir:    s.Record.WorktreePath,
		Env:    s.paneEnv(p.ID),
		Cols:   cols,
		Rows:   rows,
		Notify: notify,
	})
	if err != nil {
		return nil, err
	}
	tab.Term = tp

	bindir := executableDir()
	go func(cmd string) {
		time.Sleep(500 * time.Millisecond)
		if bindir != "" {
			tp.TypeCommand(pathExportLine(bindir))
			time.Sleep(150 * time.Millisecond)
		}
		if cmd != "" {
			time.Sleep(250 * time.Millisecond)
			tp.TypeCommand(cmd)
		}
	}(command)

	p.Tabs = append(p.Tabs, tab)
	p.Active = len(p.Tabs) - 1
	return tab, nil
}

// pathExportLine prepends dir to PATH in the syntax of the user's shell.
// Login shells on some platforms rewrite PATH during startup, so the
// environment-level value alone is not reliable.
func pathExportLine(dir string) string {
	if strings.Contains(shellx.LoginShell(), "fish") {
		return "set -x PATH '" + dir + "' $PATH"
	}
	return "export PATH='" + dir + "':\"$PATH\""
}

// buildTree constructs the pane tree from a layout node. Tabs nodes become a
// single pane with multiple tabs; visit leaves become link tabs.
func (s *Session) buildTree(n *layout.Node, cols, rows int) *TreeNode {
	if n == nil {
		n = &layout.Node{Kind: layout.KindRun}
	}
	switch n.Kind {
	case layout.KindSplit:
		return &TreeNode{
			Dir:    n.Dir,
			Ratio:  n.Ratio(),
			First:  s.buildTree(n.First, cols, rows),
			Second: s.buildTree(n.Second, cols, rows),
		}
	case layout.KindTabs:
		pane := &Pane{ID: newID(), Seq: s.app.paneSeq.next()}
		for _, c := range n.Children {
			s.addLeafTab(pane, c, cols, rows)
		}
		if len(pane.Tabs) > 0 {
			pane.Active = 0
		}
		return leafNode(pane)
	default:
		pane := &Pane{ID: newID(), Seq: s.app.paneSeq.next()}
		s.addLeafTab(pane, n, cols, rows)
		return leafNode(pane)
	}
}

func (s *Session) addLeafTab(pane *Pane, n *layout.Node, cols, rows int) {
	if n.Kind == layout.KindVisit {
		pane.Tabs = append(pane.Tabs, &Tab{ID: newID(), Seq: s.app.tabSeq.next(), URL: n.URL})
		return
	}
	if _, err := s.newTerminalTab(pane, n.Command, cols, rows); err != nil {
		// Represent the failure as a link-style tab so the pane is not
		// silently empty.
		pane.Tabs = append(pane.Tabs, &Tab{ID: newID(), Seq: s.app.tabSeq.next(), URL: "error: " + err.Error()})
	}
}

// open builds the session's panes from its layout, rewriting agent commands
// to resume prior conversations.
func (s *Session) open(app *App, cols, rows int) error {
	s.app = app
	if err := s.startServer(app); err != nil {
		return err
	}
	node, _ := layout.Parse(layout.Dedent(s.Record.LayoutText))
	layout.RewriteRunCommands(node, func(cmd string) string {
		return claudex.RewriteRunCommand(cmd, s.Record.ClaudeSessionID, s.Record.WorktreePath)
	})
	s.Tree = s.buildTree(node, cols, rows)
	panes := s.Tree.Panes()
	if len(panes) > 0 {
		s.FocusedID = panes[0].ID
	}
	return nil
}

// close terminates every pane's process and releases the socket.
func (s *Session) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, p := range s.Tree.Panes() {
		for _, t := range p.Tabs {
			if t.Term != nil {
				_ = t.Term.Terminate(ctx)
			}
		}
	}
	if s.server != nil {
		_ = s.server.Close()
	}
}

// FocusedPane returns the focused pane, or nil.
func (s *Session) FocusedPane() *Pane {
	if s.Tree == nil {
		return nil
	}
	for _, p := range s.Tree.Panes() {
		if p.ID == s.FocusedID {
			return p
		}
	}
	return nil
}

// findPane resolves a pane by seq override, pane ID, or first pane.
func (s *Session) findPane(paneSeq *int, paneID string) *Pane {
	panes := s.Tree.Panes()
	if len(panes) == 0 {
		return nil
	}
	if paneSeq != nil {
		for _, p := range panes {
			if p.Seq == *paneSeq {
				return p
			}
		}
	}
	if paneID != "" {
		for _, p := range panes {
			if p.ID == paneID {
				return p
			}
		}
	}
	return panes[0]
}

// refreshGitStatus recomputes branch, commit, and diff numbers. It runs off
// the UI goroutine.
func (s *Session) refreshGitStatus() gitStatusMsg {
	msg := gitStatusMsg{worktree: s.Record.WorktreePath}
	dir := s.Record.WorktreePath
	if branch, err := gitx.CurrentBranch(dir); err == nil {
		msg.branch = branch
	}
	if sha, err := gitx.ShortSHA(dir); err == nil {
		msg.sha = sha
	}
	msg.add, msg.del, _ = gitx.DiffStats(dir)
	return msg
}
