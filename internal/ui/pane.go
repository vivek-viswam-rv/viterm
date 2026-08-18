package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/vivek-viswam-rv/viterm/internal/term"
)

// newID returns a random identifier for panes and tabs.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", b)
	}
	return hex.EncodeToString(b[:])
}

// seqCounter numbers panes and tabs in creation order for the current run.
type seqCounter struct{ n int }

func (c *seqCounter) next() int {
	c.n++
	return c.n
}

// Tab is one tab of a pane: a terminal or a link.
type Tab struct {
	ID  string
	Seq int

	// Term is set for terminal tabs.
	Term *term.Pane
	// Command is the command line the terminal tab was started with, empty
	// for a plain shell.
	Command string

	// URL is set for link tabs.
	URL string
}

// IsTerminal reports whether the tab hosts a terminal.
func (t *Tab) IsTerminal() bool { return t.URL == "" }

// Title returns the label shown in the tab bar.
func (t *Tab) Title() string {
	if !t.IsTerminal() {
		if u, err := url.Parse(t.URL); err == nil && u.Host != "" {
			return u.Host
		}
		return t.URL
	}
	if t.Term != nil {
		if title := strings.TrimSpace(t.Term.Title()); title != "" {
			return title
		}
	}
	if fields := strings.Fields(t.Command); len(fields) > 0 {
		return fields[0]
	}
	return "shell"
}

// Pane is a tabbed slot in the split tree.
type Pane struct {
	ID     string
	Seq    int
	Tabs   []*Tab
	Active int
}

// ActiveTab returns the selected tab, or nil for an empty pane.
func (p *Pane) ActiveTab() *Tab {
	if p.Active < 0 || p.Active >= len(p.Tabs) {
		return nil
	}
	return p.Tabs[p.Active]
}

// SelectTab clamps and selects a tab index.
func (p *Pane) SelectTab(i int) {
	if len(p.Tabs) == 0 {
		p.Active = 0
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(p.Tabs) {
		i = len(p.Tabs) - 1
	}
	p.Active = i
}

// CycleTab moves the selection by delta, wrapping.
func (p *Pane) CycleTab(delta int) {
	if len(p.Tabs) == 0 {
		return
	}
	p.Active = ((p.Active+delta)%len(p.Tabs) + len(p.Tabs)) % len(p.Tabs)
}

// RemoveTab deletes the tab at index i and adjusts the selection.
func (p *Pane) RemoveTab(i int) {
	if i < 0 || i >= len(p.Tabs) {
		return
	}
	p.Tabs = append(p.Tabs[:i], p.Tabs[i+1:]...)
	if p.Active >= len(p.Tabs) {
		p.Active = len(p.Tabs) - 1
	}
	if p.Active < 0 {
		p.Active = 0
	}
}

// FindTab locates a tab by ID or sequence number string.
func (p *Pane) FindTab(idOrSeq string) (int, *Tab) {
	for i, t := range p.Tabs {
		if t.ID == idOrSeq || fmt.Sprintf("%d", t.Seq) == idOrSeq {
			return i, t
		}
	}
	return -1, nil
}
