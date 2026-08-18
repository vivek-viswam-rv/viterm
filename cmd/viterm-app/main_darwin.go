//go:build darwin

// Command viterm-app is the macOS window host for viterm. It opens a native
// window with an embedded terminal surface, applies the theme and font from
// viterm's settings, and runs the viterm application inside it. The look of
// the window is fully controlled by viterm's configuration rather than by
// any terminal application.
package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	webview "github.com/webview/webview_go"

	"github.com/vivek-viswam-rv/viterm/internal/config"
	"github.com/vivek-viswam-rv/viterm/internal/theme"
)

//go:embed assets/xterm.js
var xtermJS string

//go:embed assets/xterm.css
var xtermCSS string

//go:embed assets/addon-fit.js
var fitJS string

const (
	defaultFont     = "SF Mono, Menlo, Monaco, monospace"
	defaultFontSize = 13
)

func main() {
	settings := loadSettings()
	th := theme.Get(settings.Theme)

	binary, err := findViterm()
	if err != nil {
		fatalDialog(err.Error())
	}

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"VITERM_HOST=app",
	)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 34})
	if err != nil {
		fatalDialog("starting viterm: " + err.Error())
	}

	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("viterm")
	w.SetSize(1200, 800, webview.HintNone)

	var mu sync.Mutex
	closed := false
	closeOnce := func() {
		mu.Lock()
		defer mu.Unlock()
		if closed {
			return
		}
		closed = true
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
		_ = ptmx.Close()
	}

	_ = w.Bind("vitermInput", func(b64 string) {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			_, _ = ptmx.Write(data)
		}
	})
	_ = w.Bind("vitermResize", func(cols, rows int) {
		if cols > 0 && rows > 0 {
			_ = pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
		}
	})
	_ = w.Bind("vitermReady", func() {
		go pump(w, ptmx, &mu, &closed)
	})

	go func() {
		_ = cmd.Wait()
		closeOnce()
		w.Dispatch(w.Terminate)
	}()

	w.SetHtml(page(settings, th))
	w.Run()
	closeOnce()
}

// pump forwards terminal output into the page as base64 chunks.
func pump(w webview.WebView, ptmx *os.File, mu *sync.Mutex, closed *bool) {
	buf := make([]byte, 32*1024)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			b64 := base64.StdEncoding.EncodeToString(buf[:n])
			mu.Lock()
			stop := *closed
			mu.Unlock()
			if stop {
				return
			}
			w.Dispatch(func() { w.Eval("vitermWrite('" + b64 + "')") })
		}
		if err != nil {
			return
		}
	}
}

func loadSettings() config.Settings {
	store, err := config.Open()
	if err != nil {
		return config.Settings{}
	}
	settings, err := store.Settings()
	if err != nil {
		return config.Settings{}
	}
	return settings
}

// findViterm locates the viterm binary: bundled next to the host first,
// then the standard install location, then PATH.
func findViterm() (string, error) {
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "viterm")
		if info, err := os.Stat(bundled); err == nil && !info.IsDir() {
			return bundled, nil
		}
	}
	if info, err := os.Stat("/usr/local/bin/viterm"); err == nil && !info.IsDir() {
		return "/usr/local/bin/viterm", nil
	}
	if path, err := exec.LookPath("viterm"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("the viterm binary was not found; reinstall the package")
}

// fatalDialog shows the error in a window, since the host has no terminal
// to print to.
func fatalDialog(msg string) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("viterm")
	w.SetSize(520, 200, webview.HintFixed)
	w.SetHtml("<body style='font-family:-apple-system;background:#1a1b26;color:#c0caf5;" +
		"display:flex;align-items:center;justify-content:center'><p>" + msg + "</p></body>")
	w.Run()
	os.Exit(1)
}

// page renders the host page with the theme and font applied.
func page(settings config.Settings, th theme.Theme) string {
	font := settings.Font
	if font == "" {
		font = defaultFont
	}
	size := settings.FontSize
	if size <= 0 {
		size = defaultFontSize
	}

	xtermTheme := map[string]string{
		"background":          th.Terminal.Background,
		"foreground":          th.Terminal.Foreground,
		"cursor":              th.Terminal.Cursor,
		"selectionBackground": th.Terminal.Selection,
		"black":               th.Terminal.ANSI[0],
		"red":                 th.Terminal.ANSI[1],
		"green":               th.Terminal.ANSI[2],
		"yellow":              th.Terminal.ANSI[3],
		"blue":                th.Terminal.ANSI[4],
		"magenta":             th.Terminal.ANSI[5],
		"cyan":                th.Terminal.ANSI[6],
		"white":               th.Terminal.ANSI[7],
		"brightBlack":         th.Terminal.ANSI[8],
		"brightRed":           th.Terminal.ANSI[9],
		"brightGreen":         th.Terminal.ANSI[10],
		"brightYellow":        th.Terminal.ANSI[11],
		"brightBlue":          th.Terminal.ANSI[12],
		"brightMagenta":       th.Terminal.ANSI[13],
		"brightCyan":          th.Terminal.ANSI[14],
		"brightWhite":         th.Terminal.ANSI[15],
	}
	themeJSON, _ := json.Marshal(xtermTheme)
	fontJSON, _ := json.Marshal(font)

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><style>`)
	b.WriteString(xtermCSS)
	b.WriteString(`html,body{margin:0;padding:0;height:100%;overflow:hidden;background:`)
	b.WriteString(th.Terminal.Background)
	b.WriteString(`}#term{height:100%;padding:6px;box-sizing:border-box}`)
	b.WriteString(`</style></head><body><div id="term"></div><script>`)
	b.WriteString(xtermJS)
	b.WriteString("\n")
	b.WriteString(fitJS)
	b.WriteString(`
const term = new Terminal({
  fontFamily: ` + string(fontJSON) + `,
  fontSize: ` + fmt.Sprintf("%d", size) + `,
  theme: ` + string(themeJSON) + `,
  cursorBlink: true,
  allowProposedApi: true,
  macOptionIsMeta: true,
  scrollback: 0,
});
const fit = new FitAddon.FitAddon();
term.loadAddon(fit);
term.open(document.getElementById("term"));
fit.fit();
term.focus();

function toB64(s) {
  const bytes = new TextEncoder().encode(s);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}
function vitermWrite(b64) {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  term.write(bytes);
}
term.onData((data) => { vitermInput(toB64(data)); });
term.onBinary((data) => { vitermInput(btoa(data)); });
term.onResize(({cols, rows}) => { vitermResize(cols, rows); });
window.addEventListener("resize", () => fit.fit());
vitermResize(term.cols, term.rows);
vitermReady();
</script></body></html>`)
	return b.String()
}
