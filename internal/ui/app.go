// Package ui implements the viterm terminal application: sessions rendered
// as a strip of tabs, each holding a split tree of PTY-backed panes, driven
// by a prefix key so that ordinary keystrokes always reach the programs
// running inside the panes.
package ui

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/vivekviswam/viterm/internal/claudex"
	"github.com/vivekviswam/viterm/internal/config"
	"github.com/vivekviswam/viterm/internal/ghx"
	"github.com/vivekviswam/viterm/internal/gitx"
	"github.com/vivekviswam/viterm/internal/ipc"
	"github.com/vivekviswam/viterm/internal/layout"
	"github.com/vivekviswam/viterm/internal/protocol"
	"github.com/vivekviswam/viterm/internal/shellx"
	"github.com/vivekviswam/viterm/internal/term"
)

const (
	gitPollInterval = 3 * time.Second
	prPollInterval  = 30 * time.Second
)

// App is the root model.
type App struct {
	store    *config.Store
	settings config.Settings

	sessions []*Session
	active   int

	width, height int
	restored      bool

	prefixPending bool
	copyMode      bool
	confirmDelete bool
	showHelp      bool

	wizard *wizard

	paneSeq seqCounter
	tabSeq  seqCounter

	program *tea.Program
	err     string
}

// New builds the application model.
func New(store *config.Store) (*App, error) {
	settings, err := store.Settings()
	if err != nil {
		return nil, err
	}
	return &App{store: store, settings: settings}, nil
}

// SetProgram attaches the running program so background goroutines can send
// messages into the update loop. It must be called before Run.
func (a *App) SetProgram(p *tea.Program) { a.program = p }

func (a *App) send(msg tea.Msg) {
	if a.program != nil {
		a.program.Send(msg)
	}
}

// notifyFunc adapts a pane's event callback into an update-loop message.
func (a *App) notifyFunc(s *Session, tabID string) func(term.Event) {
	return func(ev term.Event) {
		a.send(paneEventMsg{session: s, tabID: tabID, event: ev})
	}
}

// Init schedules the status pollers and cleans up stale sockets.
func (a *App) Init() tea.Cmd {
	_ = ipc.CleanStale()
	return tea.Batch(
		tea.Tick(gitPollInterval, func(time.Time) tea.Msg { return gitTickMsg{} }),
		tea.Tick(prPollInterval, func(time.Time) tea.Msg { return prTickMsg{} }),
	)
}

func (a *App) activeSession() *Session {
	if a.active < 0 || a.active >= len(a.sessions) {
		return nil
	}
	return a.sessions[a.active]
}

// bodyRect is the area available to the pane tree: everything between the
// session strip and the footer.
func (a *App) bodyRect() Rect {
	h := a.height - 2
	if h < 1 {
		h = 1
	}
	return Rect{X: 0, Y: 1, W: a.width, H: h}
}

// currentRects computes the visible pane rectangles of the active session.
func (a *App) currentRects() []PaneRect {
	s := a.activeSession()
	if s == nil || s.Tree == nil {
		return nil
	}
	body := a.bodyRect()
	if s.Maximized != "" {
		if leaf := s.Tree.findLeaf(s.Maximized); leaf != nil {
			return []PaneRect{{Pane: leaf.Pane, Rect: body}}
		}
	}
	return s.Tree.Layout(body)
}

// resizePanes brings every visible pane's terminals to their laid-out size.
func (a *App) resizePanes() {
	for _, pr := range a.currentRects() {
		cols, rows := pr.Rect.W, pr.Rect.H-1
		for _, t := range pr.Pane.Tabs {
			if t.Term != nil {
				t.Term.Resize(cols, rows)
			}
		}
	}
}

// paneSize returns the terminal size for new tabs of a pane.
func (a *App) paneSize(paneID string) (int, int) {
	for _, pr := range a.currentRects() {
		if pr.Pane.ID == paneID {
			return pr.Rect.W, pr.Rect.H - 1
		}
	}
	body := a.bodyRect()
	return body.W, body.H - 1
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.width <= 0 || a.height <= 0 {
			// Some environments report no size; fall back to a usable one.
			a.width, a.height = 80, 24
		}
		if !a.restored {
			a.restored = true
			a.restoreSessions()
		}
		a.resizePanes()
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case tea.PasteMsg:
		if a.wizard != nil {
			a.wizard.paste(msg.Content)
			return a, nil
		}
		if p := a.focusedTerminal(); p != nil {
			p.Paste(msg.Content)
		}
		return a, nil

	case tea.MouseClickMsg:
		a.handleClick(tea.Mouse(msg))
		return a, nil

	case tea.MouseWheelMsg:
		a.handleWheel(tea.Mouse(msg))
		return a, nil

	case paneEventMsg:
		return a.handlePaneEvent(msg)

	case sockCmdMsg:
		a.handleSocket(msg)
		return a, nil

	case gitTickMsg:
		cmds := []tea.Cmd{tea.Tick(gitPollInterval, func(time.Time) tea.Msg { return gitTickMsg{} })}
		if s := a.activeSession(); s != nil {
			sess := s
			cmds = append(cmds, func() tea.Msg { return sess.refreshGitStatus() })
		}
		return a, tea.Batch(cmds...)

	case prTickMsg:
		cmds := []tea.Cmd{tea.Tick(prPollInterval, func(time.Time) tea.Msg { return prTickMsg{} })}
		if ghx.Available() {
			for _, s := range a.sessions {
				worktree := s.Record.WorktreePath
				cmds = append(cmds, func() tea.Msg { return resolvePR(worktree) })
			}
		}
		return a, tea.Batch(cmds...)

	case gitStatusMsg:
		for _, s := range a.sessions {
			if s.Record.WorktreePath == msg.worktree {
				s.Branch, s.ShortSHA, s.DiffAdd, s.DiffDel = msg.branch, msg.sha, msg.add, msg.del
			}
		}
		return a, nil

	case prStatusMsg:
		if msg.found {
			for _, s := range a.sessions {
				if s.Record.WorktreePath == msg.worktree {
					pr := &config.PRInfo{
						Number: msg.pr.Number, Title: msg.pr.Title, URL: msg.pr.URL,
						State: msg.pr.State, IsDraft: msg.pr.IsDraft,
					}
					s.PR = pr
					s.Record.PR = pr
					_ = a.store.SetSessionPR(s.Record.WorktreePath, pr)
				}
			}
		}
		return a, nil

	case sessionCreatedMsg:
		return a.handleSessionCreated(msg)
	}
	return a, nil
}

// resolvePR runs off the UI goroutine.
func resolvePR(worktree string) tea.Msg {
	branch, err := gitx.CurrentBranch(worktree)
	if err != nil {
		return prStatusMsg{worktree: worktree}
	}
	sha, _ := shellx.Run(worktree, "git", "rev-parse", "HEAD")
	pr, found := ghx.Resolve(worktree, branch, sha)
	return prStatusMsg{worktree: worktree, pr: pr, found: found}
}

// restoreSessions reopens every attached session record, or opens the wizard
// when none exist.
func (a *App) restoreSessions() {
	records, err := a.store.Sessions()
	if err != nil {
		a.err = err.Error()
	}
	body := a.bodyRect()
	for _, rec := range records {
		if !rec.Attached {
			continue
		}
		s := &Session{Record: rec}
		if err := s.open(a, body.W, body.H-1); err != nil {
			a.err = err.Error()
			continue
		}
		a.sessions = append(a.sessions, s)
	}
	if len(a.sessions) == 0 {
		a.openWizard()
	} else {
		a.active = 0
		a.resizePanes()
	}
}

func (a *App) openWizard() {
	a.wizard = newWizard(a)
}

// focusedTerminal returns the focused pane's active terminal, or nil.
func (a *App) focusedTerminal() *term.Pane {
	s := a.activeSession()
	if s == nil {
		return nil
	}
	p := s.FocusedPane()
	if p == nil {
		return nil
	}
	t := p.ActiveTab()
	if t == nil || t.Term == nil {
		return nil
	}
	return t.Term
}

func toUVKey(k tea.KeyPressMsg) uv.KeyPressEvent {
	return uv.KeyPressEvent{
		Text:        k.Text,
		Mod:         uv.KeyMod(k.Mod),
		Code:        k.Code,
		ShiftedCode: k.ShiftedCode,
		BaseCode:    k.BaseCode,
		IsRepeat:    k.IsRepeat,
	}
}

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}
	if a.confirmDelete {
		a.confirmDelete = false
		if msg.Keystroke() == "y" || msg.Keystroke() == "shift+y" {
			return a, a.deleteActiveSession()
		}
		return a, nil
	}
	if a.wizard != nil {
		return a, a.wizard.handleKey(msg)
	}
	if a.copyMode {
		a.handleCopyModeKey(msg)
		return a, nil
	}
	if a.prefixPending {
		a.prefixPending = false
		return a.handlePrefixKey(msg)
	}
	if msg.Keystroke() == a.settings.Prefix {
		a.prefixPending = true
		return a, nil
	}
	if p := a.focusedTerminal(); p != nil {
		p.SendKey(toUVKey(msg))
	}
	return a, nil
}

func (a *App) handleCopyModeKey(msg tea.KeyPressMsg) {
	p := a.focusedTerminal()
	if p == nil {
		a.copyMode = false
		return
	}
	_, rows := 0, a.bodyRect().H-1
	switch msg.Keystroke() {
	case "q", "esc":
		a.copyMode = false
		for p.ScrollOffset() > 0 {
			p.ScrollBy(-p.ScrollOffset())
		}
	case "k", "up":
		p.ScrollBy(1)
	case "j", "down":
		p.ScrollBy(-1)
	case "u", "pgup", "b":
		p.ScrollBy(rows / 2)
	case "d", "pgdown", "f":
		p.ScrollBy(-rows / 2)
	case "g":
		p.ScrollBy(1 << 24)
	case "shift+g", "G":
		p.ScrollBy(-(1 << 24))
	}
}

func (a *App) handlePrefixKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	action, arg := lookupPrefixAction(msg, a.settings.Prefix)
	s := a.activeSession()

	switch action {
	case actionSendPrefix:
		if p := a.focusedTerminal(); p != nil {
			p.SendKey(toUVKey(msg))
		}
	case actionHelp:
		a.showHelp = true
	case actionQuit:
		return a, a.quit()
	case actionNewSession:
		a.openWizard()
	}
	if s == nil {
		return a, nil
	}

	switch action {
	case actionNewTab:
		if p := s.FocusedPane(); p != nil {
			cols, rows := a.paneSize(p.ID)
			if _, err := s.newTerminalTab(p, "", cols, rows); err != nil {
				a.err = err.Error()
			}
		}
	case actionCloseTab:
		a.closeActiveTab(s)
	case actionNextTab:
		if p := s.FocusedPane(); p != nil {
			p.CycleTab(1)
		}
	case actionPrevTab:
		if p := s.FocusedPane(); p != nil {
			p.CycleTab(-1)
		}
	case actionSelectTab:
		if p := s.FocusedPane(); p != nil {
			p.SelectTab(arg)
		}
	case actionSplitRight:
		a.splitFocused(s, layout.DirColumns)
	case actionSplitDown:
		a.splitFocused(s, layout.DirRows)
	case actionClosePane:
		a.closeFocusedPane(s)
	case actionFocusLeft:
		a.moveFocus(s, -1, 0)
	case actionFocusDown:
		a.moveFocus(s, 0, 1)
	case actionFocusUp:
		a.moveFocus(s, 0, -1)
	case actionFocusRight:
		a.moveFocus(s, 1, 0)
	case actionMaximize:
		if s.Maximized == "" {
			s.Maximized = s.FocusedID
		} else {
			s.Maximized = ""
		}
		a.resizePanes()
	case actionCopyMode:
		if a.focusedTerminal() != nil {
			a.copyMode = true
		}
	case actionClear:
		if p := a.focusedTerminal(); p != nil {
			p.ClearScreen()
		}
	case actionDiffOpen:
		a.openDiffView(s)
	case actionDiffClose:
		a.closeDiffView(s)
	case actionDetach:
		a.detachActiveSession()
	case actionDelete:
		a.confirmDelete = true
	case actionNextSession:
		if len(a.sessions) > 0 {
			a.selectSession((a.active + 1) % len(a.sessions))
		}
	case actionPrevSession:
		if len(a.sessions) > 0 {
			a.selectSession((a.active - 1 + len(a.sessions)) % len(a.sessions))
		}
	case actionOpenURL:
		a.openStatusURL(s)
	}
	return a, nil
}

func (a *App) selectSession(i int) {
	if i < 0 || i >= len(a.sessions) {
		return
	}
	a.active = i
	a.sessions[i].Tint = ""
	a.resizePanes()
}

func (a *App) splitFocused(s *Session, dir layout.Dir) {
	p := s.FocusedPane()
	if p == nil {
		return
	}
	newPane := &Pane{ID: newID(), Seq: a.paneSeq.next()}
	if !s.Tree.SplitPane(p.ID, dir, newPane) {
		return
	}
	s.FocusedID = newPane.ID
	s.Maximized = ""
	a.resizePanes()
	cols, rows := a.paneSize(newPane.ID)
	if _, err := s.newTerminalTab(newPane, "", cols, rows); err != nil {
		a.err = err.Error()
	}
	a.resizePanes()
}

func (a *App) closeActiveTab(s *Session) {
	p := s.FocusedPane()
	if p == nil {
		return
	}
	t := p.ActiveTab()
	if t == nil {
		return
	}
	if t.Term != nil {
		go terminateTab(t)
	}
	p.RemoveTab(p.Active)
	if len(p.Tabs) == 0 {
		a.closeFocusedPane(s)
	}
}

func (a *App) closeFocusedPane(s *Session) {
	p := s.FocusedPane()
	if p == nil {
		return
	}
	for _, t := range p.Tabs {
		if t.Term != nil {
			go terminateTab(t)
		}
	}
	if s.Maximized == p.ID {
		s.Maximized = ""
	}
	if !s.Tree.RemovePane(p.ID) {
		// Last pane: closing it detaches the session.
		a.detachActiveSession()
		return
	}
	if panes := s.Tree.Panes(); len(panes) > 0 {
		s.FocusedID = panes[0].ID
	}
	a.resizePanes()
}

func (a *App) moveFocus(s *Session, dx, dy int) {
	if s.Maximized != "" {
		return
	}
	if next := FocusNeighbor(a.currentRects(), s.FocusedID, dx, dy); next != nil {
		s.FocusedID = next.ID
	}
}

func (a *App) openDiffView(s *Session) {
	if s.DiffTabID != "" {
		return
	}
	panes := s.Tree.Panes()
	if len(panes) == 0 {
		return
	}
	p := panes[len(panes)-1]
	cols, rows := a.paneSize(p.ID)
	tab, err := s.newTerminalTab(p, a.settings.DiffCommand, cols, rows)
	if err != nil {
		a.err = err.Error()
		return
	}
	s.DiffTabID = tab.ID
	s.DiffPaneID = p.ID
	s.FocusedID = p.ID
	s.Maximized = p.ID
	a.resizePanes()
}

func (a *App) closeDiffView(s *Session) {
	if s.DiffTabID == "" {
		return
	}
	if s.Maximized == s.DiffPaneID {
		s.Maximized = ""
	}
	for _, p := range s.Tree.Panes() {
		if p.ID != s.DiffPaneID {
			continue
		}
		if i, t := p.FindTab(s.DiffTabID); t != nil {
			if t.Term != nil {
				go terminateTab(t)
			}
			p.RemoveTab(i)
		}
	}
	s.DiffTabID, s.DiffPaneID = "", ""
	a.resizePanes()
}

func (a *App) openStatusURL(s *Session) {
	if s.PR != nil && s.PR.URL != "" {
		_ = shellx.OpenInBrowser(s.PR.URL)
		return
	}
	if url, ok := gitx.CommitURL(s.Record.WorktreePath); ok {
		_ = shellx.OpenInBrowser(url)
	}
}

func terminateTab(t *Tab) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = t.Term.Terminate(ctx)
}

// detachActiveSession closes the session but keeps it listed for reattach.
func (a *App) detachActiveSession() {
	s := a.activeSession()
	if s == nil {
		return
	}
	_ = a.store.SetSessionAttached(s.Record.WorktreePath, false)
	go s.close()
	a.sessions = append(a.sessions[:a.active], a.sessions[a.active+1:]...)
	if a.active >= len(a.sessions) {
		a.active = len(a.sessions) - 1
	}
	if a.active < 0 {
		a.active = 0
	}
	if len(a.sessions) == 0 {
		a.openWizard()
	} else {
		a.resizePanes()
	}
}

// deleteActiveSession removes the session, its worktree, and its agent
// transcripts.
func (a *App) deleteActiveSession() tea.Cmd {
	s := a.activeSession()
	if s == nil {
		return nil
	}
	worktree := s.Record.WorktreePath
	repoPath := a.repoPathFor(s.Record.RepoName)
	_ = a.store.RemoveSession(worktree)
	go s.close()
	a.sessions = append(a.sessions[:a.active], a.sessions[a.active+1:]...)
	if a.active >= len(a.sessions) {
		a.active = len(a.sessions) - 1
	}
	if a.active < 0 {
		a.active = 0
	}
	if len(a.sessions) == 0 {
		a.openWizard()
	} else {
		a.resizePanes()
	}
	return func() tea.Msg {
		if repoPath != "" {
			_ = gitx.RemoveWorktree(repoPath, worktree)
		}
		_ = claudex.RemoveProject(worktree)
		return nil
	}
}

func (a *App) repoPathFor(name string) string {
	repos, err := a.store.Repos()
	if err != nil {
		return ""
	}
	for _, r := range repos {
		if r.Name == name {
			return r.Path
		}
	}
	return ""
}

// quit closes every session's processes and exits. Sessions stay attached so
// the next launch restores them.
func (a *App) quit() tea.Cmd {
	sessions := a.sessions
	return func() tea.Msg {
		for _, s := range sessions {
			s.close()
		}
		return tea.QuitMsg{}
	}
}

func (a *App) handlePaneEvent(msg paneEventMsg) (tea.Model, tea.Cmd) {
	switch msg.event.(type) {
	case term.ExitEvent:
		for si, s := range a.sessions {
			if s != msg.session {
				continue
			}
			for _, p := range s.Tree.Panes() {
				if i, t := p.FindTab(msg.tabID); t != nil {
					if s.DiffTabID == t.ID {
						s.DiffTabID, s.DiffPaneID = "", ""
						if s.Maximized == p.ID {
							s.Maximized = ""
						}
					}
					p.RemoveTab(i)
					if len(p.Tabs) == 0 {
						if !s.Tree.RemovePane(p.ID) {
							// Last pane of the session exited.
							if si == a.active {
								a.detachActiveSession()
							}
							return a, nil
						}
						if s.FocusedID == p.ID {
							if panes := s.Tree.Panes(); len(panes) > 0 {
								s.FocusedID = panes[0].ID
							}
						}
					}
					a.resizePanes()
					return a, nil
				}
			}
		}
	}
	// Update and title events simply trigger a repaint.
	return a, nil
}

func (a *App) handleSocket(msg sockCmdMsg) {
	s := msg.session
	reply := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			b = []byte(`{"ok":false,"error":"encoding reply"}`)
		}
		msg.reply <- b
	}

	switch msg.cmd.Action {
	case protocol.ActionTabsList:
		var entries []protocol.TabListEntry
		for _, p := range s.Tree.Panes() {
			for i, t := range p.Tabs {
				typ := "terminal"
				if !t.IsTerminal() {
					typ = "link"
				}
				entries = append(entries, protocol.TabListEntry{
					TabID:    t.ID,
					TabSeq:   t.Seq,
					PaneID:   p.ID,
					PaneSeq:  p.Seq,
					Type:     typ,
					Title:    t.Title(),
					IsActive: i == p.Active && p.ID == s.FocusedID,
				})
			}
		}
		if entries == nil {
			entries = []protocol.TabListEntry{}
		}
		reply(entries)

	case protocol.ActionTabSend:
		for _, p := range s.Tree.Panes() {
			if _, t := p.FindTab(msg.cmd.TabID); t != nil {
				if t.Term == nil {
					reply(protocol.Reply{OK: false, Error: "tab is not a terminal: " + msg.cmd.TabID})
					return
				}
				t.Term.SendText(msg.cmd.Text)
				reply(protocol.Reply{OK: true})
				return
			}
		}
		reply(protocol.Reply{OK: false, Error: "tab not found: " + msg.cmd.TabID})

	case protocol.ActionTerminalRun:
		p := s.findPane(msg.cmd.PaneSeq, msg.cmd.PaneID)
		if p == nil {
			reply(protocol.Reply{OK: false, Error: "no panes"})
			return
		}
		cols, rows := a.paneSize(p.ID)
		if _, err := s.newTerminalTab(p, msg.cmd.Command, cols, rows); err != nil {
			reply(protocol.Reply{OK: false, Error: err.Error()})
			return
		}
		a.resizePanes()
		reply(protocol.Reply{OK: true})

	case protocol.ActionURLOpen:
		p := s.findPane(msg.cmd.PaneSeq, msg.cmd.PaneID)
		if p == nil {
			reply(protocol.Reply{OK: false, Error: "no panes"})
			return
		}
		tab := &Tab{ID: newID(), Seq: a.tabSeq.next(), URL: msg.cmd.URL}
		p.Tabs = append(p.Tabs, tab)
		background := msg.cmd.Background != nil && *msg.cmd.Background
		if !background {
			p.Active = len(p.Tabs) - 1
		}
		_ = shellx.OpenInBrowser(msg.cmd.URL)
		reply(protocol.Reply{OK: true})

	case protocol.ActionSessionNotify:
		a.applyTint(s, msg.cmd.Command)
		reply(protocol.Reply{OK: true})

	case protocol.ActionClaudeEvent:
		switch msg.cmd.Command {
		case protocol.ClaudeEventStop:
			a.applyTint(s, protocol.ColorGreen)
		case protocol.ClaudeEventPermission:
			a.applyTint(s, protocol.ColorRed)
		case protocol.ClaudeEventPromptSubmit:
			a.applyTint(s, protocol.ColorClear)
		}
		if id := msg.cmd.ClaudeSessionID; id != "" && id != s.Record.ClaudeSessionID {
			s.Record.ClaudeSessionID = id
			_ = a.store.SetClaudeSessionID(s.Record.WorktreePath, id)
		}
		reply(protocol.Reply{OK: true})

	default:
		reply(protocol.Reply{OK: false, Error: "unknown action: " + msg.cmd.Action})
	}
}

// applyTint sets a session's notification color. A completion tint on the
// session the user is already looking at is dropped.
func (a *App) applyTint(s *Session, color string) {
	switch color {
	case protocol.ColorClear, "none", "reset":
		s.Tint = ""
		return
	case protocol.ColorGreen, protocol.ColorRed, protocol.ColorYellow,
		protocol.ColorBlue, protocol.ColorOrange:
	default:
		color = protocol.ColorGreen
	}
	if color == protocol.ColorGreen && a.activeSession() == s {
		return
	}
	s.Tint = color
}

func (a *App) handleClick(m tea.Mouse) {
	s := a.activeSession()
	if s == nil || a.wizard != nil {
		return
	}
	for _, pr := range a.currentRects() {
		r := pr.Rect
		if m.X < r.X || m.X >= r.X+r.W || m.Y < r.Y || m.Y >= r.Y+r.H {
			continue
		}
		s.FocusedID = pr.Pane.ID
		if m.Y == r.Y {
			a.selectTabAt(pr.Pane, m.X-r.X)
			return
		}
		if t := pr.Pane.ActiveTab(); t != nil && t.Term != nil && t.Term.IsAltScreen() {
			ev := uv.Mouse{X: m.X - r.X, Y: m.Y - r.Y - 1, Button: m.Button, Mod: uv.KeyMod(m.Mod)}
			t.Term.SendMouse(uv.MouseClickEvent(ev))
		}
		return
	}
}

// selectTabAt maps a click x offset within the tab bar to a tab index.
func (a *App) selectTabAt(p *Pane, x int) {
	pos := 0
	for i, t := range p.Tabs {
		w := len(tabLabel(i, t)) + 1
		if x < pos+w {
			p.SelectTab(i)
			return
		}
		pos += w
	}
}

func (a *App) handleWheel(m tea.Mouse) {
	s := a.activeSession()
	if s == nil || a.wizard != nil {
		return
	}
	for _, pr := range a.currentRects() {
		r := pr.Rect
		if m.X < r.X || m.X >= r.X+r.W || m.Y < r.Y || m.Y >= r.Y+r.H {
			continue
		}
		t := pr.Pane.ActiveTab()
		if t == nil || t.Term == nil {
			return
		}
		delta := 3
		if m.Button == tea.MouseWheelDown {
			delta = -3
		}
		if t.Term.IsAltScreen() {
			ev := uv.Mouse{X: m.X - r.X, Y: m.Y - r.Y - 1, Button: m.Button, Mod: uv.KeyMod(m.Mod)}
			t.Term.SendMouse(uv.MouseWheelEvent(ev))
			return
		}
		t.Term.ScrollBy(delta)
		return
	}
}

func (a *App) handleSessionCreated(msg sessionCreatedMsg) (tea.Model, tea.Cmd) {
	if a.wizard != nil {
		a.wizard.creating = false
	}
	if msg.err != nil {
		if a.wizard != nil {
			a.wizard.errText = msg.err.Error()
		} else {
			a.err = msg.err.Error()
		}
		return a, nil
	}
	rec := config.SessionRecord{
		WorktreePath: msg.worktreePath,
		RepoName:     msg.repoName,
		SessionName:  msg.name,
		Slug:         msg.slug,
		LayoutText:   msg.layoutText,
		Attached:     true,
	}
	_ = a.store.UpsertSession(rec)
	s := &Session{Record: rec}
	body := a.bodyRect()
	if err := s.open(a, body.W, body.H-1); err != nil {
		a.err = err.Error()
		return a, nil
	}
	a.sessions = append(a.sessions, s)
	a.active = len(a.sessions) - 1
	a.wizard = nil
	a.resizePanes()
	return a, nil
}

// attachSession reopens a detached session record.
func (a *App) attachSession(rec config.SessionRecord) {
	rec.Attached = true
	_ = a.store.SetSessionAttached(rec.WorktreePath, true)
	s := &Session{Record: rec}
	body := a.bodyRect()
	if err := s.open(a, body.W, body.H-1); err != nil {
		a.err = err.Error()
		return
	}
	a.sessions = append(a.sessions, s)
	a.active = len(a.sessions) - 1
	a.wizard = nil
	a.resizePanes()
}

// sessionTitle is the strip label for a session.
func sessionTitle(s *Session) string {
	name := s.Record.SessionName
	if name == "" {
		name = s.Record.Slug
	}
	return strings.TrimSpace(name)
}
