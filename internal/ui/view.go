package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/vivekviswam/viterm/internal/config"
	"github.com/vivekviswam/viterm/internal/layout"
	"github.com/vivekviswam/viterm/internal/protocol"
	"github.com/vivekviswam/viterm/internal/theme"
)

var (
	colorAccent = lipgloss.Color("12")
	colorDim    = lipgloss.Color("8")
	colorFg     = lipgloss.Color("15")
	tintPalette = map[string]color.Color{
		protocol.ColorGreen:  lipgloss.Color("10"),
		protocol.ColorRed:    lipgloss.Color("9"),
		protocol.ColorYellow: lipgloss.Color("11"),
		protocol.ColorBlue:   lipgloss.Color("12"),
		protocol.ColorOrange: lipgloss.Color("208"),
	}

	styleStripActive   = lipgloss.NewStyle().Bold(true).Foreground(colorFg).Background(lipgloss.Color("237"))
	styleStripInactive = lipgloss.NewStyle().Foreground(colorDim)
	styleTabActive     = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleTabInactive   = lipgloss.NewStyle().Foreground(colorDim)
	styleFooter        = lipgloss.NewStyle().Foreground(colorDim)
	styleFooterAlert   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	styleDivider       = lipgloss.NewStyle().Foreground(colorDim)
)

// configureStyles applies the configured theme and color overrides to the
// chrome styles. With no theme and no overrides the adaptive ANSI defaults
// above are kept, so the chrome follows the hosting terminal's palette.
func configureStyles(cfg config.Settings) {
	if cfg.Theme == "" && len(cfg.Colors) == 0 && os.Getenv("VITERM_HOST") == "" {
		return
	}
	t := theme.Get(cfg.Theme)
	accent := lipgloss.Color(pick(cfg.Colors, "accent", t.Chrome.Accent))
	dim := lipgloss.Color(pick(cfg.Colors, "dim", t.Chrome.Dim))
	fg := lipgloss.Color(pick(cfg.Colors, "foreground", t.Chrome.Foreground))
	stripBg := lipgloss.Color(pick(cfg.Colors, "stripBackground", t.Chrome.StripBackground))

	colorAccent, colorDim, colorFg = accent, dim, fg
	styleStripActive = lipgloss.NewStyle().Bold(true).Foreground(fg).Background(stripBg)
	styleStripInactive = lipgloss.NewStyle().Foreground(dim)
	styleTabActive = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleTabInactive = lipgloss.NewStyle().Foreground(dim)
	styleFooter = lipgloss.NewStyle().Foreground(dim)
	styleFooterAlert = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleDivider = lipgloss.NewStyle().Foreground(dim)
}

func pick(overrides map[string]string, key, fallback string) string {
	if v, ok := overrides[key]; ok && v != "" {
		return v
	}
	return fallback
}

// View implements tea.Model.
func (a *App) View() tea.View {
	var view tea.View
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "viterm"

	if a.width == 0 || a.height == 0 {
		view.Content = "starting..."
		return view
	}
	if a.wizard != nil {
		view.Content = a.wizard.view(a.width, a.height)
		return view
	}
	if a.showHelp {
		view.Content = a.helpView()
		return view
	}

	s := a.activeSession()
	if s == nil {
		view.Content = "no sessions"
		return view
	}

	strip := a.renderStrip()
	body := a.renderBody(s)
	footer := a.renderFooter(s)
	view.Content = strip + "\n" + body + "\n" + footer

	if cur := a.cursorFor(s); cur != nil {
		view.Cursor = cur
	}
	return view
}

// cursorFor places the real terminal cursor over the focused pane's cursor.
func (a *App) cursorFor(s *Session) *tea.Cursor {
	if a.copyMode || a.prefixPending || a.confirmDelete {
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
	x, y, visible := t.Term.CursorPos()
	if !visible {
		return nil
	}
	for _, pr := range a.currentRects() {
		if pr.Pane.ID != p.ID {
			continue
		}
		cx, cy := pr.Rect.X+x, pr.Rect.Y+1+y
		if cx >= pr.Rect.X+pr.Rect.W || cy >= pr.Rect.Y+pr.Rect.H {
			return nil
		}
		return tea.NewCursor(cx, cy)
	}
	return nil
}

func (a *App) renderStrip() string {
	var parts []string
	for i, s := range a.sessions {
		label := fmt.Sprintf(" %d:%s ", i+1, sessionTitle(s))
		style := styleStripInactive
		if i == a.active {
			style = styleStripActive
		}
		if c, ok := tintPalette[s.Tint]; ok {
			style = style.Foreground(c)
		}
		parts = append(parts, style.Render(label))
	}
	line := strings.Join(parts, styleDivider.Render("│"))
	return padLine(line, a.width)
}

func (a *App) renderBody(s *Session) string {
	body := a.bodyRect()
	if s.Maximized != "" {
		if leaf := s.Tree.findLeaf(s.Maximized); leaf != nil {
			return a.renderLeaf(leaf.Pane, body.W, body.H, s)
		}
	}
	return a.renderNode(s.Tree, body.W, body.H, s)
}

func (a *App) renderNode(n *TreeNode, w, h int, s *Session) string {
	if n == nil {
		return strings.TrimRight(strings.Repeat(strings.Repeat(" ", w)+"\n", h), "\n")
	}
	if n.IsLeaf() {
		return a.renderLeaf(n.Pane, w, h, s)
	}
	ratio := n.Ratio
	if ratio <= 0 {
		ratio = 0.5
	}
	if n.Dir == layout.DirColumns {
		avail := w - 1
		first := clamp(int(float64(avail)*ratio), 1, avail-1)
		left := a.renderNode(n.First, first, h, s)
		right := a.renderNode(n.Second, avail-first, h, s)
		divider := strings.TrimRight(strings.Repeat(styleDivider.Render("│")+"\n", h), "\n")
		return lipgloss.JoinHorizontal(lipgloss.Top, left, divider, right)
	}
	avail := h - 1
	first := clamp(int(float64(avail)*ratio), 1, avail-1)
	top := a.renderNode(n.First, w, first, s)
	bottom := a.renderNode(n.Second, w, avail-first, s)
	divider := styleDivider.Render(strings.Repeat("─", w))
	return top + "\n" + divider + "\n" + bottom
}

func tabLabel(i int, t *Tab) string {
	return fmt.Sprintf(" %d %s ", i+1, t.Title())
}

func (a *App) renderLeaf(p *Pane, w, h int, s *Session) string {
	focused := p.ID == s.FocusedID
	var bar strings.Builder
	for i, t := range p.Tabs {
		style := styleTabInactive
		if i == p.Active {
			if focused {
				style = styleTabActive
			} else {
				style = styleTabActive.Bold(false)
			}
		}
		bar.WriteString(style.Render(tabLabel(i, t)))
		bar.WriteString(" ")
	}
	lines := make([]string, 0, h)
	lines = append(lines, padLine(ansi.Truncate(bar.String(), w, "…"), w))

	content := ""
	if t := p.ActiveTab(); t != nil {
		if t.Term != nil {
			content = t.Term.Render()
		} else {
			content = a.renderLinkTab(t, w, h-1)
		}
	}
	body := strings.Split(content, "\n")
	for i := 0; i < h-1; i++ {
		if i < len(body) {
			lines = append(lines, padLine(ansi.Truncate(body[i], w, ""), w))
		} else {
			lines = append(lines, strings.Repeat(" ", w))
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) renderLinkTab(t *Tab, w, h int) string {
	msg := t.URL + "\n\n" + "enter: open in browser"
	return lipgloss.NewStyle().
		Width(w).Height(h).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(colorAccent).
		Render(msg)
}

func (a *App) renderFooter(s *Session) string {
	var left []string
	if s.Branch != "" {
		left = append(left, s.Branch)
	}
	if s.DiffAdd > 0 || s.DiffDel > 0 {
		left = append(left, fmt.Sprintf("+%d -%d", s.DiffAdd, s.DiffDel))
	}
	if s.ShortSHA != "" {
		left = append(left, s.ShortSHA)
	}
	if s.PR != nil {
		state := strings.ToLower(s.PR.State)
		if s.PR.IsDraft {
			state = "draft"
		}
		left = append(left, fmt.Sprintf("PR #%d %s", s.PR.Number, state))
	}

	right := ""
	switch {
	case a.confirmDelete:
		right = "delete session and worktree? y/N"
	case a.copyMode:
		off := 0
		if t := a.focusedTerminal(); t != nil {
			off = t.ScrollOffset()
		}
		right = fmt.Sprintf("copy mode [%d] q:exit", off)
	case a.prefixPending:
		right = "prefix..."
	case a.err != "":
		right = a.err
	default:
		right = a.settings.Prefix + " ? for help"
	}

	leftStr := styleFooter.Render(strings.Join(left, " · "))
	rightStyle := styleFooter
	if a.confirmDelete || a.copyMode || a.prefixPending || a.err != "" {
		rightStyle = styleFooterAlert
	}
	rightStr := rightStyle.Render(right)

	gap := a.width - lipgloss.Width(leftStr) - lipgloss.Width(rightStr)
	if gap < 1 {
		gap = 1
	}
	return ansi.Truncate(leftStr+strings.Repeat(" ", gap)+rightStr, a.width, "")
}

func (a *App) helpView() string {
	p := a.settings.Prefix
	rows := [][2]string{
		{"c", "new terminal tab"}, {"x", "close tab"},
		{"n / p / 1-9", "switch tab"}, {"v / s", "split right / down"},
		{"X", "close pane"}, {"h j k l", "focus pane"},
		{"z", "maximize pane"}, {"[", "scrollback"},
		{"L", "clear terminal"}, {"g / G", "open / close diff"},
		{"N", "new session"}, {"d", "detach session"},
		{"D", "delete session"}, {"tab / shift+tab", "switch session"},
		{"o", "open PR or commit page"}, {"Q", "quit"},
		{p, "send the prefix itself"},
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("viterm — every command starts with "+p) + "\n\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %-18s %s\n", r[0], r[1]))
	}
	b.WriteString("\npress any key to close")
	return lipgloss.NewStyle().Width(a.width).Height(a.height).
		Align(lipgloss.Center, lipgloss.Center).Render(b.String())
}

// padLine pads a styled line to an exact display width.
func padLine(line string, w int) string {
	width := lipgloss.Width(line)
	if width >= w {
		return ansi.Truncate(line, w, "")
	}
	return line + strings.Repeat(" ", w-width)
}
