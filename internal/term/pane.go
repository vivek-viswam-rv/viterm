// Package term hosts a shell process on a pseudo-terminal and emulates the
// terminal it writes to, exposing a styled snapshot of the screen for
// rendering inside the viterm UI. Input events are encoded for the child
// process according to the modes the running application has negotiated
// (application cursor keys, bracketed paste, mouse reporting), so full-screen
// programs behave as they would in a standalone terminal.
package term

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// Event is a notification emitted by a pane. Events are delivered on the
// Notify callback from internal goroutines; receivers must not block.
type Event any

// UpdateEvent signals that the screen content changed and a repaint is due.
// Updates are coalesced to at most one event per frame interval.
type UpdateEvent struct{}

// TitleEvent signals that the application changed the terminal title.
type TitleEvent struct{ Title string }

// BellEvent signals a terminal bell.
type BellEvent struct{}

// ExitEvent signals that the child process exited. Err carries the wait
// error, if any.
type ExitEvent struct{ Err error }

// Options configures a new pane.
type Options struct {
	// Shell is the executable to run; it is started as a login shell.
	Shell string
	// Dir is the working directory for the child process.
	Dir string
	// Env holds extra KEY=VALUE entries appended to the parent environment.
	Env []string
	// Cols and Rows give the initial terminal size.
	Cols, Rows int
	// Scrollback caps the number of scrollback lines (0 keeps the default).
	Scrollback int
	// Notify receives pane events. It is called from internal goroutines and
	// must be safe for concurrent use and non-blocking.
	Notify func(Event)
}

const frameInterval = 16 * time.Millisecond

// Pane is a single terminal: a PTY-backed child process plus the emulated
// screen it renders to.
type Pane struct {
	emu  *vt.SafeEmulator
	ptmx *os.File
	cmd  *exec.Cmd

	notify func(Event)

	mu         sync.Mutex
	title      string
	altScreen  bool
	cursorHide bool
	scrollOff  int
	exited     bool

	dirty     chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

// Start launches the shell on a new PTY and begins pumping output into the
// emulator and encoded input back to the child.
func Start(opts Options) (*Pane, error) {
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}
	if opts.Notify == nil {
		opts.Notify = func(Event) {}
	}

	cmd := exec.Command(opts.Shell, "-l")
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), opts.Env...)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(opts.Rows),
		Cols: uint16(opts.Cols),
	})
	if err != nil {
		return nil, fmt.Errorf("starting shell %q: %w", opts.Shell, err)
	}

	emu := vt.NewSafeEmulator(opts.Cols, opts.Rows)
	if opts.Scrollback > 0 {
		emu.SetScrollbackSize(opts.Scrollback)
	}

	p := &Pane{
		emu:    emu,
		ptmx:   ptmx,
		cmd:    cmd,
		notify: opts.Notify,
		dirty:  make(chan struct{}, 1),
		closed: make(chan struct{}),
	}

	emu.SetCallbacks(vt.Callbacks{
		Title: func(t string) {
			p.mu.Lock()
			p.title = t
			p.mu.Unlock()
			p.notify(TitleEvent{Title: t})
		},
		Bell: func() { p.notify(BellEvent{}) },
		AltScreen: func(on bool) {
			p.mu.Lock()
			p.altScreen = on
			p.scrollOff = 0
			p.mu.Unlock()
		},
		CursorVisibility: func(visible bool) {
			p.mu.Lock()
			p.cursorHide = !visible
			p.mu.Unlock()
		},
	})

	go p.pumpOutput()
	go p.pumpInput()
	go p.coalesceUpdates()
	go p.wait()

	return p, nil
}

// pumpOutput copies child output into the emulator and marks the screen
// dirty.
func (p *Pane) pumpOutput() {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.ptmx.Read(buf)
		if n > 0 {
			_, _ = p.emu.Write(buf[:n])
			select {
			case p.dirty <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// pumpInput forwards input bytes produced by the emulator to the child.
func (p *Pane) pumpInput() {
	_, _ = io.Copy(p.ptmx, p.emu)
}

// coalesceUpdates rate-limits repaint notifications to one per frame.
func (p *Pane) coalesceUpdates() {
	for {
		select {
		case <-p.closed:
			return
		case <-p.dirty:
			p.notify(UpdateEvent{})
			select {
			case <-p.closed:
				return
			case <-time.After(frameInterval):
			}
		}
	}
}

// wait reaps the child process and reports its exit.
func (p *Pane) wait() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.exited = true
	p.mu.Unlock()
	debugf("pane shell pid=%d exited: %v", p.cmd.Process.Pid, err)
	p.notify(ExitEvent{Err: err})
}

// debugf appends to the file named by VITERM_DEBUG_LOG when set.
func debugf(format string, args ...any) {
	path := os.Getenv("VITERM_DEBUG_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, format+"\n", args...)
}

// Resize adjusts both the PTY and the emulated screen.
func (p *Pane) Resize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	p.mu.Lock()
	p.scrollOff = 0
	p.mu.Unlock()
	_ = pty.Setsize(p.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	p.emu.Resize(cols, rows)
}

// SendKey encodes a key press for the child according to the modes the
// running application has set.
func (p *Pane) SendKey(k uv.KeyPressEvent) {
	p.resetScroll()
	p.emu.SendKey(k)
}

// SendMouse forwards a mouse event. The emulator drops it unless the running
// application enabled mouse reporting.
func (p *Pane) SendMouse(m uv.MouseEvent) {
	p.emu.SendMouse(m)
}

// SendText writes raw text to the child as if typed.
func (p *Pane) SendText(s string) {
	p.resetScroll()
	p.emu.SendText(s)
}

// TypeCommand types a command line into the shell and submits it.
func (p *Pane) TypeCommand(cmd string) {
	p.SendText(cmd + "\r")
}

// Paste inserts text, bracketing it when the running application enabled
// bracketed paste mode.
func (p *Pane) Paste(s string) {
	p.resetScroll()
	p.emu.Paste(s)
}

// IsAltScreen reports whether the alternate screen is active.
func (p *Pane) IsAltScreen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.altScreen
}

// Exited reports whether the child process has exited.
func (p *Pane) Exited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

// Title returns the most recent title set by the application.
func (p *Pane) Title() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.title
}

// ScrollBy moves the scrollback viewport by delta lines (positive scrolls
// toward older content) and reports whether the offset changed.
func (p *Pane) ScrollBy(delta int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.altScreen {
		return false
	}
	off := p.scrollOff + delta
	if off < 0 {
		off = 0
	}
	if max := p.emu.ScrollbackLen(); off > max {
		off = max
	}
	changed := off != p.scrollOff
	p.scrollOff = off
	return changed
}

// ScrollOffset returns the current scrollback offset (0 means live view).
func (p *Pane) ScrollOffset() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scrollOff
}

func (p *Pane) resetScroll() {
	p.mu.Lock()
	p.scrollOff = 0
	p.mu.Unlock()
}

// ClearScreen clears the emulator's scrollback and asks the shell to redraw
// its prompt.
func (p *Pane) ClearScreen() {
	p.emu.ClearScrollback()
	p.emu.SendText("\x0c") // Ctrl-L
}

// Render returns the visible screen as ANSI-styled lines. When the viewport
// is scrolled back it shows the corresponding slice of history.
func (p *Pane) Render() string {
	p.mu.Lock()
	off := p.scrollOff
	p.mu.Unlock()

	if off == 0 {
		return p.emu.Render()
	}

	rows := p.emu.Height()
	sbLen := p.emu.ScrollbackLen()
	if off > sbLen {
		off = sbLen
	}
	screen := strings.Split(p.emu.Render(), "\n")
	sb := p.emu.Scrollback()

	lines := make([]string, 0, rows)
	// The combined history is scrollback lines followed by the live screen;
	// the viewport ends `off` lines above the bottom.
	start := sbLen - off
	for i := 0; i < rows; i++ {
		j := start + i
		if j < sbLen {
			lines = append(lines, sb.Line(j).Render())
		} else if k := j - sbLen; k < len(screen) {
			lines = append(lines, screen[k])
		} else {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// CursorPos returns the cursor position and whether it should be drawn.
func (p *Pane) CursorPos() (x, y int, visible bool) {
	p.mu.Lock()
	hide := p.cursorHide || p.scrollOff > 0
	p.mu.Unlock()
	pos := p.emu.CursorPosition()
	return pos.X, pos.Y, !hide
}

// Terminate stops the child process, escalating from a SIGTERM to the
// foreground process group up to a SIGKILL, then releases the PTY. It is
// safe to call more than once: the PTY is released only the first time, and
// later calls return nil immediately once the process has exited.
func (p *Pane) Terminate(ctx context.Context) error {
	defer func() {
		p.closeOnce.Do(func() {
			close(p.closed)
			_ = p.emu.Close()
			_ = p.ptmx.Close()
		})
	}()

	if p.Exited() {
		return nil
	}

	// Signal the foreground process group first so an interactive program
	// (rather than the shell) receives the termination request.
	if pgid, err := unix.IoctlGetInt(int(p.ptmx.Fd()), unix.TIOCGPGRP); err == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
	if p.waitExit(ctx, 2*time.Second) {
		return nil
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
	if p.waitExit(ctx, 2*time.Second) {
		return nil
	}

	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		_ = p.cmd.Process.Kill()
	}
	return nil
}

// waitExit polls for child exit up to the given timeout or context cancel.
func (p *Pane) waitExit(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.Exited() {
			return true
		}
		select {
		case <-ctx.Done():
			return p.Exited()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return p.Exited()
}
