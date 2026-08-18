// Package shellx runs external commands for viterm: git and gh invocations,
// arbitrary user commands through a login shell, and opening URLs in the
// system browser.
package shellx

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// LoginShell returns the user's shell, falling back to a platform default.
func LoginShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}

// ExitError describes a failed command with both output streams attached so
// callers can inspect or surface them.
type ExitError struct {
	Cmd    string
	Stdout string
	Stderr string
	Err    error
}

func (e *ExitError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %v", e.Cmd, e.Err)
	if s := strings.TrimSpace(e.Stderr); s != "" {
		fmt.Fprintf(&b, ": %s", s)
	} else if s := strings.TrimSpace(e.Stdout); s != "" {
		fmt.Fprintf(&b, ": %s", s)
	}
	return b.String()
}

func (e *ExitError) Unwrap() error { return e.Err }

// Output returns whichever stream carries the failure detail, trimmed.
func (e *ExitError) Output() string {
	if s := strings.TrimSpace(e.Stderr); s != "" {
		return s
	}
	return strings.TrimSpace(e.Stdout)
}

// Run executes a command in dir and returns its trimmed stdout. On failure
// the returned error is an *ExitError carrying both output streams.
func Run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &ExitError{
			Cmd:    name + " " + strings.Join(args, " "),
			Stdout: stdout.String(),
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunShell executes a command line through the user's login shell in dir,
// with extra KEY=VALUE env entries appended, returning trimmed combined
// output.
func RunShell(dir string, env []string, command string) (string, error) {
	sh := LoginShell()
	cmd := exec.Command(sh, "-l", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", &ExitError{
			Cmd:    sh + " -l -c " + command,
			Stdout: out.String(),
			Err:    err,
		}
	}
	return strings.TrimSpace(out.String()), nil
}

// OpenInBrowser opens an http(s) URL with the platform's default handler.
func OpenInBrowser(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("refusing to open non-http(s) URL %q", rawURL)
	}
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, rawURL).Start()
}
