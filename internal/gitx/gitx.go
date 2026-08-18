// Package gitx implements viterm's git operations: worktree-per-session
// lifecycle, name sanitization, default-branch detection, and lightweight
// status queries. Everything shells out to the git CLI so user hooks and
// configuration behave exactly as they do in a normal checkout.
package gitx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/vivekviswam/viterm/internal/shellx"
)

// MaxWorktreeNameLength caps worktree directory (and branch) names.
const MaxWorktreeNameLength = 30

// SanitizeName converts a free-form session name into a filesystem- and
// branch-safe slug. Spaces and ".." become "-", and any character other than
// a letter, digit, '-', '_', '/', or '.' is dropped. Length is not capped
// here; UniqueWorktreeName applies the cap.
func SanitizeName(name string) string {
	s := strings.ReplaceAll(name, " ", "-")
	s = strings.ReplaceAll(s, "..", "-")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) ||
			r == '-' || r == '_' || r == '/' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// UniqueWorktreeName sanitizes name and returns a variant that does not
// collide with an existing entry under baseDir, appending -1, -2, ... while
// keeping the total length within MaxWorktreeNameLength.
func UniqueWorktreeName(baseDir, name string) string {
	sanitized := SanitizeName(name)
	if sanitized == "" {
		return ""
	}
	base := truncateRunes(sanitized, MaxWorktreeNameLength)
	if _, err := os.Stat(filepath.Join(baseDir, base)); err != nil {
		return base
	}
	for i := 1; ; i++ {
		suffix := fmt.Sprintf("-%d", i)
		candidate := truncateRunes(sanitized, MaxWorktreeNameLength-len(suffix)) + suffix
		if _, err := os.Stat(filepath.Join(baseDir, candidate)); err != nil {
			return candidate
		}
	}
}

// DefaultBranch detects the repository's default branch: the target of
// origin/HEAD when set, else "main" if such a ref exists, else "master".
func DefaultBranch(repoPath string) string {
	if out, err := shellx.Run(repoPath, "git", "symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		parts := strings.Split(strings.TrimSpace(out), "/")
		if len(parts) > 0 && parts[len(parts)-1] != "" {
			return parts[len(parts)-1]
		}
	}
	if _, err := shellx.Run(repoPath, "git", "rev-parse", "--verify", "main"); err == nil {
		return "main"
	}
	return "master"
}

// CreateWorktreeOptions configures CreateWorktree.
type CreateWorktreeOptions struct {
	// RepoPath is the primary checkout the worktree is created from.
	RepoPath string
	// BaseDir is the root under which worktrees are grouped by repo name.
	BaseDir string
	// RepoName groups this repo's worktrees under BaseDir.
	RepoName string
	// Name is the sanitized, collision-free worktree name; it is also used
	// as the branch name.
	Name string
	// Pull updates the primary checkout's default branch before branching.
	Pull bool
	// PostCreateCommand, when non-empty, runs through the login shell with
	// WORKTREE_DIRECTORY set. Failures are ignored.
	PostCreateCommand string
}

// CreateWorktree creates <BaseDir>/<RepoName>/<Name> as a new worktree
// branched from the default branch. An already-existing directory is reused
// as-is. A worktree that git registered despite a failing checkout hook is
// treated as created.
func CreateWorktree(opts CreateWorktreeOptions) (worktreePath, slug string, err error) {
	slug = opts.Name
	worktreePath = filepath.Join(opts.BaseDir, opts.RepoName, slug)

	if _, statErr := os.Stat(worktreePath); statErr == nil {
		runPostCreate(opts, worktreePath)
		return worktreePath, slug, nil
	}

	defaultBranch := DefaultBranch(opts.RepoPath)

	if opts.Pull {
		stashOut, stashErr := shellx.Run(opts.RepoPath, "git", "stash", "--include-untracked")
		didStash := stashErr == nil && !strings.Contains(stashOut, "No local changes")
		_, _ = shellx.Run(opts.RepoPath, "git", "checkout", defaultBranch)
		if _, pullErr := shellx.Run(opts.RepoPath, "git", "pull"); pullErr != nil {
			if didStash {
				_, _ = shellx.Run(opts.RepoPath, "git", "stash", "pop")
			}
			return "", "", fmt.Errorf("git pull on %s failed: %s", defaultBranch, errOutput(pullErr))
		}
		if didStash {
			_, _ = shellx.Run(opts.RepoPath, "git", "stash", "pop")
		}
	}

	if mkErr := os.MkdirAll(filepath.Join(opts.BaseDir, opts.RepoName), 0o755); mkErr != nil {
		return "", "", fmt.Errorf("creating worktree base directory: %w", mkErr)
	}

	_, err1 := shellx.Run(opts.RepoPath, "git", "worktree", "add", "-b", slug, worktreePath, defaultBranch)
	if err1 == nil || isRegisteredWorktree(opts.RepoPath, worktreePath) {
		runPostCreate(opts, worktreePath)
		return worktreePath, slug, nil
	}

	_, err2 := shellx.Run(opts.RepoPath, "git", "worktree", "add", worktreePath, slug)
	if err2 == nil || isRegisteredWorktree(opts.RepoPath, worktreePath) {
		runPostCreate(opts, worktreePath)
		return worktreePath, slug, nil
	}

	return "", "", fmt.Errorf("creating worktree failed: %s / %s", errOutput(err1), errOutput(err2))
}

func errOutput(err error) string {
	var xerr *shellx.ExitError
	if errors.As(err, &xerr) {
		if out := xerr.Output(); out != "" {
			return out
		}
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

func runPostCreate(opts CreateWorktreeOptions, worktreePath string) {
	cmd := strings.TrimSpace(opts.PostCreateCommand)
	if cmd == "" {
		return
	}
	_, _ = shellx.RunShell(opts.RepoPath, []string{"WORKTREE_DIRECTORY=" + worktreePath}, cmd)
}

func isRegisteredWorktree(repoPath, worktreePath string) bool {
	out, err := shellx.Run(repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "worktree "+worktreePath {
			return true
		}
	}
	return false
}

// RemoveWorktree detaches and deletes a worktree directory, falling back to
// a direct removal, and prunes stale worktree metadata. It fails only when
// the directory still exists afterwards.
func RemoveWorktree(repoPath, worktreePath string) error {
	_, _ = shellx.Run(repoPath, "git", "worktree", "remove", "--force", worktreePath)
	if _, err := os.Stat(worktreePath); err == nil {
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			return fmt.Errorf("removing worktree directory: %w", rmErr)
		}
	}
	_, _ = shellx.Run(repoPath, "git", "worktree", "prune")
	return nil
}

// DiffStats sums added and deleted line counts of uncommitted changes.
// Binary entries are counted as zero.
func DiffStats(dir string) (added, deleted int, err error) {
	out, err := shellx.Run(dir, "git", "diff", "--numstat", "HEAD")
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		a, _ := strconv.Atoi(fields[0])
		d, _ := strconv.Atoi(fields[1])
		added += a
		deleted += d
	}
	return added, deleted, nil
}

// ShortSHA returns the abbreviated HEAD commit hash.
func ShortSHA(dir string) (string, error) {
	return shellx.Run(dir, "git", "rev-parse", "--short", "HEAD")
}

// CurrentBranch returns the checked-out branch name.
func CurrentBranch(dir string) (string, error) {
	return shellx.Run(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
}

// RemoteWebURL derives the https://github.com/owner/repo page for the origin
// remote, when origin points at GitHub.
func RemoteWebURL(dir string) (string, bool) {
	out, err := shellx.Run(dir, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", false
	}
	return remoteWebURL(out)
}

func remoteWebURL(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	var path string
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		path = strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "https://github.com/"):
		path = strings.TrimPrefix(remote, "https://github.com/")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		path = strings.TrimPrefix(remote, "ssh://git@github.com/")
	default:
		return "", false
	}
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	if path == "" {
		return "", false
	}
	return "https://github.com/" + path, true
}

// CommitURL returns the GitHub page for the current HEAD commit.
func CommitURL(dir string) (string, bool) {
	base, ok := RemoteWebURL(dir)
	if !ok {
		return "", false
	}
	sha, err := shellx.Run(dir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", false
	}
	return base + "/commit/" + sha, true
}
