package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivekviswam/viterm/internal/shellx"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"my feature", "my-feature"},
		{"a..b", "a-b"},
		{"weird!@#chars", "weirdchars"},
		{"path/like_name.v2", "path/like_name.v2"},
		{"héllo wörld", "héllo-wörld"},
		{strings.Repeat("x", 50), strings.Repeat("x", 50)}, // no truncation here
	}
	for _, tt := range tests {
		if got := SanitizeName(tt.in); got != tt.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestUniqueWorktreeName(t *testing.T) {
	base := t.TempDir()

	long := strings.Repeat("a", 40)
	got := UniqueWorktreeName(base, long)
	if len([]rune(got)) != MaxWorktreeNameLength {
		t.Errorf("truncated length = %d, want %d", len([]rune(got)), MaxWorktreeNameLength)
	}

	if err := os.Mkdir(filepath.Join(base, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := UniqueWorktreeName(base, "taken"); got != "taken-1" {
		t.Errorf("first collision = %q, want %q", got, "taken-1")
	}
	if err := os.Mkdir(filepath.Join(base, "taken-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := UniqueWorktreeName(base, "taken"); got != "taken-2" {
		t.Errorf("second collision = %q, want %q", got, "taken-2")
	}

	if err := os.Mkdir(filepath.Join(base, strings.Repeat("a", 30)), 0o755); err != nil {
		t.Fatal(err)
	}
	got = UniqueWorktreeName(base, long)
	if len([]rune(got)) > MaxWorktreeNameLength {
		t.Errorf("collision suffix overflows cap: %q (%d)", got, len([]rune(got)))
	}
	if !strings.HasSuffix(got, "-1") {
		t.Errorf("collision candidate = %q, want -1 suffix", got)
	}
}

func TestRemoteWebURLParsing(t *testing.T) {
	tests := []struct {
		remote string
		want   string
		ok     bool
	}{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo", true},
		{"https://github.com/owner/repo", "https://github.com/owner/repo", true},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo", true},
		{"ssh://git@github.com/owner/repo.git", "https://github.com/owner/repo", true},
		{"git@gitlab.com:owner/repo.git", "", false},
		{"https://example.com/owner/repo", "", false},
	}
	for _, tt := range tests {
		got, ok := remoteWebURL(tt.remote)
		if got != tt.want || ok != tt.ok {
			t.Errorf("remoteWebURL(%q) = %q, %v; want %q, %v", tt.remote, got, ok, tt.want, tt.ok)
		}
	}
}

// setupRepos builds a bare origin plus a pushed clone for integration tests.
func setupRepos(t *testing.T) (originPath, clonePath string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	originPath = filepath.Join(root, "origin.git")
	clonePath = filepath.Join(root, "clone")

	mustRun(t, root, "git", "init", "--bare", "-b", "main", originPath)
	mustRun(t, root, "git", "clone", originPath, clonePath)
	mustRun(t, clonePath, "git", "config", "user.email", "test@example.com")
	mustRun(t, clonePath, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clonePath, "README"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, clonePath, "git", "add", ".")
	mustRun(t, clonePath, "git", "commit", "-m", "initial")
	mustRun(t, clonePath, "git", "push", "-u", "origin", "main")
	mustRun(t, clonePath, "git", "remote", "set-head", "origin", "--auto")
	return originPath, clonePath
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	if out, err := shellx.Run(dir, name, args...); err != nil {
		t.Fatalf("%s %v: %v (%s)", name, args, err, out)
	}
}

func TestDefaultBranchIntegration(t *testing.T) {
	_, clone := setupRepos(t)
	if got := DefaultBranch(clone); got != "main" {
		t.Errorf("DefaultBranch = %q, want main", got)
	}
}

func TestCreateWorktreeHappyPath(t *testing.T) {
	_, clone := setupRepos(t)
	base := t.TempDir()

	path, slug, err := CreateWorktree(CreateWorktreeOptions{
		RepoPath: clone,
		BaseDir:  base,
		RepoName: "proj",
		Name:     "feature-x",
	})
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if slug != "feature-x" {
		t.Errorf("slug = %q", slug)
	}
	if _, err := os.Stat(filepath.Join(path, "README")); err != nil {
		t.Errorf("worktree missing checkout: %v", err)
	}
	if branch, err := CurrentBranch(path); err != nil || branch != "feature-x" {
		t.Errorf("branch = %q, %v", branch, err)
	}
}

func TestCreateWorktreeBranchExistsRetry(t *testing.T) {
	_, clone := setupRepos(t)
	base := t.TempDir()

	mustRun(t, clone, "git", "branch", "existing")
	path, _, err := CreateWorktree(CreateWorktreeOptions{
		RepoPath: clone,
		BaseDir:  base,
		RepoName: "proj",
		Name:     "existing",
	})
	if err != nil {
		t.Fatalf("CreateWorktree with existing branch: %v", err)
	}
	if branch, _ := CurrentBranch(path); branch != "existing" {
		t.Errorf("branch = %q, want existing", branch)
	}
}

func TestCreateWorktreeReusesExistingDirectory(t *testing.T) {
	_, clone := setupRepos(t)
	base := t.TempDir()
	target := filepath.Join(base, "proj", "reused")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(base, "marker")

	path, _, err := CreateWorktree(CreateWorktreeOptions{
		RepoPath:          clone,
		BaseDir:           base,
		RepoName:          "proj",
		Name:              "reused",
		PostCreateCommand: "touch '" + marker + "'",
	})
	if err != nil {
		t.Fatalf("CreateWorktree reuse: %v", err)
	}
	if path != target {
		t.Errorf("path = %q, want %q", path, target)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("post-create command did not run on reuse")
	}
	// The directory was reused as-is, so it must not have become a checkout.
	if _, err := os.Stat(filepath.Join(target, "README")); err == nil {
		t.Error("reuse path unexpectedly ran a checkout")
	}
}

func TestRemoveWorktreeIntegration(t *testing.T) {
	_, clone := setupRepos(t)
	base := t.TempDir()
	path, _, err := CreateWorktree(CreateWorktreeOptions{
		RepoPath: clone, BaseDir: base, RepoName: "proj", Name: "gone",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(clone, path); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
}

func TestStatusQueriesIntegration(t *testing.T) {
	_, clone := setupRepos(t)

	sha, err := ShortSHA(clone)
	if err != nil || sha == "" {
		t.Errorf("ShortSHA = %q, %v", sha, err)
	}
	if branch, err := CurrentBranch(clone); err != nil || branch != "main" {
		t.Errorf("CurrentBranch = %q, %v", branch, err)
	}

	if err := os.WriteFile(filepath.Join(clone, "README"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add, del, err := DiffStats(clone)
	if err != nil {
		t.Fatalf("DiffStats: %v", err)
	}
	if add < 1 {
		t.Errorf("added = %d, want >= 1 (deleted %d)", add, del)
	}

	mustRun(t, clone, "git", "remote", "set-url", "origin", "git@github.com:owner/repo.git")
	url, ok := RemoteWebURL(clone)
	if !ok || url != "https://github.com/owner/repo" {
		t.Errorf("RemoteWebURL = %q, %v", url, ok)
	}
	curl, ok := CommitURL(clone)
	if !ok || !strings.HasPrefix(curl, "https://github.com/owner/repo/commit/") {
		t.Errorf("CommitURL = %q, %v", curl, ok)
	}
}
