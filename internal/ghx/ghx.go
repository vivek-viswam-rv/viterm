// Package ghx resolves the GitHub pull request associated with a worktree's
// branch using the gh CLI. Resolution degrades silently when gh is missing
// or unauthenticated: viterm simply shows no PR badge.
package ghx

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/vivekviswam/viterm/internal/shellx"
)

// PR describes a resolved pull request.
type PR struct {
	Number      int
	Title       string
	URL         string
	State       string // OPEN, MERGED, or CLOSED
	IsDraft     bool
	HeadRefName string
	HeadRefOID  string
}

const jsonFields = "number,title,url,state,isDraft,headRefName,headRefOid"

type prJSON struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
}

var (
	availOnce sync.Once
	avail     bool
)

// Available reports whether the gh CLI is installed. The result is cached
// for the process lifetime.
func Available() bool {
	availOnce.Do(func() {
		_, err := exec.LookPath("gh")
		avail = err == nil
	})
	return avail
}

// Resolve finds the pull request for the given branch and HEAD commit using
// three strategies in order: the checked-out branch's own PR, an open or
// closed PR whose head matches the branch, and a commit-hash search that
// requires an exact head match. It returns false when nothing acceptable is
// found or gh fails.
func Resolve(dir, branch, headSHA string) (PR, bool) {
	if out, err := shellx.Run(dir, "gh", "pr", "view", "--json", jsonFields); err == nil {
		var p prJSON
		if json.Unmarshal([]byte(out), &p) == nil && shouldAccept(p, branch, headSHA, false) {
			return toPR(p), true
		}
	}

	if branch != "" {
		if out, err := shellx.Run(dir, "gh", "pr", "list",
			"--head", branch, "--state", "all", "--limit", "20", "--json", jsonFields); err == nil {
			var prs []prJSON
			if json.Unmarshal([]byte(out), &prs) == nil {
				accepted := filterAccepted(prs, branch, headSHA, false)
				if best, ok := pickBest(accepted, headSHA); ok {
					return toPR(best), true
				}
			}
		}
	}

	if branch != "" && headSHA != "" {
		if out, err := shellx.Run(dir, "gh", "pr", "list",
			"--search", headSHA+" is:pr", "--state", "all", "--limit", "20", "--json", jsonFields); err == nil {
			var prs []prJSON
			if json.Unmarshal([]byte(out), &prs) == nil {
				var matched []prJSON
				for _, p := range prs {
					if p.HeadRefOid == headSHA {
						matched = append(matched, p)
					}
				}
				accepted := filterAccepted(matched, branch, headSHA, true)
				if best, ok := pickBest(accepted, headSHA); ok {
					return toPR(best), true
				}
			}
		}
	}

	return PR{}, false
}

func filterAccepted(prs []prJSON, branch, headSHA string, requireSHAForMerged bool) []prJSON {
	var out []prJSON
	for _, p := range prs {
		if shouldAccept(p, branch, headSHA, requireSHAForMerged) {
			out = append(out, p)
		}
	}
	return out
}

func shouldAccept(p prJSON, branch, headSHA string, requireSHAForMerged bool) bool {
	if branch == "" {
		return false
	}
	if !branchMatches(p.HeadRefName, branch) {
		return false
	}
	if requireSHAForMerged && (p.State == "MERGED" || p.State == "CLOSED") {
		return headSHA != "" && p.HeadRefOid == headSHA
	}
	return true
}

// branchMatches accepts an exact branch match or a fork-prefixed local
// branch ("owner/feature" matching a PR head of "feature").
func branchMatches(headRefName, localBranch string) bool {
	if headRefName == localBranch {
		return true
	}
	return strings.Contains(localBranch, "/") &&
		strings.HasSuffix(localBranch, "/"+headRefName)
}

func stateRank(state string) int {
	switch state {
	case "OPEN":
		return 2
	case "MERGED":
		return 1
	case "CLOSED":
		return 0
	default:
		return -1
	}
}

func pickBest(prs []prJSON, headSHA string) (prJSON, bool) {
	if len(prs) == 0 {
		return prJSON{}, false
	}
	sort.SliceStable(prs, func(i, j int) bool {
		iSHA := headSHA != "" && prs[i].HeadRefOid == headSHA
		jSHA := headSHA != "" && prs[j].HeadRefOid == headSHA
		if iSHA != jSHA {
			return iSHA
		}
		if a, b := stateRank(prs[i].State), stateRank(prs[j].State); a != b {
			return a > b
		}
		return prs[i].Number > prs[j].Number
	})
	return prs[0], true
}

func toPR(p prJSON) PR {
	return PR{
		Number:      p.Number,
		Title:       p.Title,
		URL:         p.URL,
		State:       p.State,
		IsDraft:     p.IsDraft,
		HeadRefName: p.HeadRefName,
		HeadRefOID:  p.HeadRefOid,
	}
}
