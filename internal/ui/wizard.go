package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vivekviswam/viterm/internal/config"
	"github.com/vivekviswam/viterm/internal/gitx"
	"github.com/vivekviswam/viterm/internal/layout"
)

// wizardMode is the wizard's current screen.
type wizardMode int

const (
	wizardPick wizardMode = iota
	wizardName
	wizardRepoPath
)

// pickItem is one selectable row of the wizard menu.
type pickItem struct {
	label  string
	repo   *config.RepoRecord
	detach *config.SessionRecord
	action string // "addrepo", "quit", "back"
}

// wizard drives session creation and reattachment.
type wizard struct {
	app  *App
	mode wizardMode

	items    []pickItem
	cursor   int
	input    string
	repo     config.RepoRecord
	creating bool
	errText  string
}

func newWizard(a *App) *wizard {
	w := &wizard{app: a}
	w.reload()
	return w
}

func (w *wizard) reload() {
	w.mode = wizardPick
	w.items = nil
	w.cursor = 0
	records, _ := w.app.store.Sessions()
	open := map[string]bool{}
	for _, s := range w.app.sessions {
		open[s.Record.WorktreePath] = true
	}
	for _, rec := range records {
		if open[rec.WorktreePath] {
			continue
		}
		rec := rec
		w.items = append(w.items, pickItem{
			label:  "attach  " + rec.SessionName + "  (" + rec.RepoName + ")",
			detach: &rec,
		})
	}
	repos, _ := w.app.store.Repos()
	for _, r := range repos {
		r := r
		w.items = append(w.items, pickItem{label: "new session  " + r.Name, repo: &r})
	}
	w.items = append(w.items, pickItem{label: "register a repository", action: "addrepo"})
	if len(w.app.sessions) > 0 {
		w.items = append(w.items, pickItem{label: "back", action: "back"})
	} else {
		w.items = append(w.items, pickItem{label: "quit", action: "quit"})
	}
}

func (w *wizard) paste(s string) {
	if w.mode == wizardName || w.mode == wizardRepoPath {
		w.input += strings.ReplaceAll(strings.TrimSpace(s), "\n", "")
	}
}

func (w *wizard) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if w.creating {
		return nil
	}
	if w.errText != "" {
		w.errText = ""
		return nil
	}
	switch w.mode {
	case wizardPick:
		return w.handlePickKey(msg)
	case wizardName, wizardRepoPath:
		return w.handleInputKey(msg)
	}
	return nil
}

func (w *wizard) handlePickKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "up", "k":
		if w.cursor > 0 {
			w.cursor--
		}
	case "down", "j":
		if w.cursor < len(w.items)-1 {
			w.cursor++
		}
	case "esc":
		if len(w.app.sessions) > 0 {
			w.app.wizard = nil
		}
	case "enter":
		if w.cursor >= len(w.items) {
			return nil
		}
		item := w.items[w.cursor]
		switch {
		case item.detach != nil:
			w.app.attachSession(*item.detach)
		case item.repo != nil:
			w.repo = *item.repo
			w.mode = wizardName
			w.input = ""
		case item.action == "addrepo":
			w.mode = wizardRepoPath
			w.input = ""
		case item.action == "back":
			w.app.wizard = nil
		case item.action == "quit":
			return w.app.quit()
		}
	}
	return nil
}

func (w *wizard) handleInputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		w.reload()
		return nil
	case "enter":
		text := strings.TrimSpace(w.input)
		if text == "" {
			return nil
		}
		if w.mode == wizardRepoPath {
			w.addRepo(text)
			return nil
		}
		return w.startCreate(text)
	case "backspace":
		if len(w.input) > 0 {
			r := []rune(w.input)
			w.input = string(r[:len(r)-1])
		}
		return nil
	case "ctrl+u":
		w.input = ""
		return nil
	}
	if msg.Text != "" {
		w.input += msg.Text
	}
	return nil
}

// addRepo registers a local repository path.
func (w *wizard) addRepo(path string) {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if info, err := os.Stat(filepath.Join(path, ".git")); err != nil || !info.IsDir() {
		w.errText = "not a git repository: " + path
		return
	}
	rec := config.NewRepoRecord(path, filepath.Base(path), layout.DefaultLayoutText)
	if err := w.app.store.AddRepo(rec); err != nil {
		w.errText = err.Error()
		return
	}
	w.reload()
}

// startCreate launches worktree creation in the background.
func (w *wizard) startCreate(name string) tea.Cmd {
	w.creating = true
	repo := w.repo
	settings := w.app.settings
	return func() tea.Msg {
		baseDir := filepath.Join(settings.WorktreeBaseDir, repo.Name)
		slug := gitx.UniqueWorktreeName(baseDir, name)
		if slug == "" {
			return sessionCreatedMsg{err: fmt.Errorf("name %q produces an empty slug", name)}
		}
		layoutText := repo.LayoutText
		if strings.TrimSpace(layoutText) == "" {
			layoutText = layout.DefaultLayoutText
		}
		worktree, slug, err := gitx.CreateWorktree(gitx.CreateWorktreeOptions{
			RepoPath:          repo.Path,
			BaseDir:           settings.WorktreeBaseDir,
			RepoName:          repo.Name,
			Name:              slug,
			Pull:              repo.PullBeforeWorktree,
			PostCreateCommand: settings.PostWorktreeCreateCommand,
		})
		if err != nil {
			return sessionCreatedMsg{err: err}
		}
		return sessionCreatedMsg{
			name:         name,
			repoName:     repo.Name,
			worktreePath: worktree,
			slug:         slug,
			layoutText:   layoutText,
		}
	}
}

func (w *wizard) view(width, height int) string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	dim := lipgloss.NewStyle().Foreground(colorDim)

	switch {
	case w.errText != "":
		b.WriteString(title.Render("error") + "\n\n")
		b.WriteString(w.errText + "\n\n")
		b.WriteString(dim.Render("press any key"))
	case w.creating:
		b.WriteString(title.Render("creating worktree...") + "\n\n")
		b.WriteString(dim.Render("branching from the default branch"))
	case w.mode == wizardName:
		b.WriteString(title.Render("new session  ·  "+w.repo.Name) + "\n\n")
		b.WriteString("session name: " + w.input + "▏\n\n")
		b.WriteString(dim.Render("enter: create    esc: back"))
	case w.mode == wizardRepoPath:
		b.WriteString(title.Render("register repository") + "\n\n")
		b.WriteString("path: " + w.input + "▏\n\n")
		b.WriteString(dim.Render("enter: add    esc: back"))
	default:
		b.WriteString(title.Render("viterm") + "\n\n")
		for i, item := range w.items {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == w.cursor {
				cursor = "> "
				style = style.Bold(true).Foreground(colorAccent)
			}
			b.WriteString(cursor + style.Render(item.label) + "\n")
		}
		b.WriteString("\n" + dim.Render("enter: select    j/k: move"))
	}
	return lipgloss.NewStyle().Width(width).Height(height).
		Align(lipgloss.Center, lipgloss.Center).Render(b.String())
}
