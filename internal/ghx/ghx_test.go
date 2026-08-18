package ghx

import (
	"os"
	"path/filepath"
	"testing"
)

// installFakeGH puts a shell script named gh first on PATH. The script
// receives the full argument list and can dispatch on it.
func installFakeGH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	full := "#!/bin/sh\n" + script
	if err := os.WriteFile(path, []byte(full), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestResolveStrategy1(t *testing.T) {
	installFakeGH(t, `
if [ "$1 $2" = "pr view" ]; then
  echo '{"number":12,"title":"Fix","url":"https://example.com/12","state":"OPEN","isDraft":false,"headRefName":"feature","headRefOid":"abc"}'
  exit 0
fi
exit 1
`)
	pr, ok := Resolve(t.TempDir(), "feature", "abc")
	if !ok {
		t.Fatal("Resolve found nothing")
	}
	if pr.Number != 12 || pr.State != "OPEN" {
		t.Errorf("pr = %+v", pr)
	}
}

func TestResolveStrategy2AfterBranchMismatch(t *testing.T) {
	installFakeGH(t, `
if [ "$1 $2" = "pr view" ]; then
  echo '{"number":1,"title":"Other","url":"u","state":"OPEN","isDraft":false,"headRefName":"different","headRefOid":"zzz"}'
  exit 0
fi
if [ "$1 $2" = "pr list" ] && [ "$3" = "--head" ]; then
  echo '[{"number":5,"title":"Mine","url":"u5","state":"OPEN","isDraft":false,"headRefName":"feature","headRefOid":"abc"}]'
  exit 0
fi
exit 1
`)
	pr, ok := Resolve(t.TempDir(), "feature", "abc")
	if !ok || pr.Number != 5 {
		t.Errorf("pr = %+v, ok = %v; want number 5", pr, ok)
	}
}

func TestResolveStrategy3RequiresExactSHA(t *testing.T) {
	installFakeGH(t, `
if [ "$1 $2" = "pr view" ]; then exit 1; fi
if [ "$1 $2" = "pr list" ] && [ "$3" = "--head" ]; then echo '[]'; exit 0; fi
if [ "$1 $2" = "pr list" ] && [ "$3" = "--search" ]; then
  echo '[{"number":9,"title":"Merged","url":"u9","state":"MERGED","isDraft":false,"headRefName":"feature","headRefOid":"abc"},{"number":10,"title":"Stale","url":"u10","state":"MERGED","isDraft":false,"headRefName":"feature","headRefOid":"other"}]'
  exit 0
fi
exit 1
`)
	pr, ok := Resolve(t.TempDir(), "feature", "abc")
	if !ok || pr.Number != 9 {
		t.Errorf("pr = %+v, ok = %v; want number 9 (exact SHA)", pr, ok)
	}
}

func TestResolveRanking(t *testing.T) {
	installFakeGH(t, `
if [ "$1 $2" = "pr view" ]; then exit 1; fi
if [ "$1 $2" = "pr list" ] && [ "$3" = "--head" ]; then
  echo '[{"number":100,"title":"Closed","url":"a","state":"CLOSED","isDraft":false,"headRefName":"feature","headRefOid":"x"},{"number":2,"title":"OpenSHA","url":"b","state":"OPEN","isDraft":false,"headRefName":"feature","headRefOid":"abc"},{"number":50,"title":"Open","url":"c","state":"OPEN","isDraft":false,"headRefName":"feature","headRefOid":"y"}]'
  exit 0
fi
exit 1
`)
	pr, ok := Resolve(t.TempDir(), "feature", "abc")
	if !ok || pr.Number != 2 {
		t.Errorf("pr = %+v; want the SHA match (number 2) despite lower number", pr)
	}
}

func TestResolveForkPrefixBranch(t *testing.T) {
	installFakeGH(t, `
if [ "$1 $2" = "pr view" ]; then
  echo '{"number":3,"title":"Fork","url":"u","state":"OPEN","isDraft":false,"headRefName":"feature","headRefOid":"abc"}'
  exit 0
fi
exit 1
`)
	pr, ok := Resolve(t.TempDir(), "owner/feature", "abc")
	if !ok || pr.Number != 3 {
		t.Errorf("fork-prefix branch did not match: %+v, %v", pr, ok)
	}
}

func TestResolveGHFailure(t *testing.T) {
	installFakeGH(t, "exit 1\n")
	if pr, ok := Resolve(t.TempDir(), "feature", "abc"); ok {
		t.Errorf("Resolve = %+v, want not found on gh failure", pr)
	}
}

func TestBranchMatches(t *testing.T) {
	tests := []struct {
		head, local string
		want        bool
	}{
		{"feature", "feature", true},
		{"feature", "owner/feature", true},
		{"feature", "other", false},
		{"feature", "featurex", false},
		{"feature", "owner:feature", false},
	}
	for _, tt := range tests {
		if got := branchMatches(tt.head, tt.local); got != tt.want {
			t.Errorf("branchMatches(%q, %q) = %v, want %v", tt.head, tt.local, got, tt.want)
		}
	}
}
