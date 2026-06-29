package state

import "time"

// State represents where a managed fork sits in the patch pipeline.
type State int

const (
	// Uninitialized: no PATCHER.md. Needs /patch-init and human setup.
	Uninitialized State = iota
	// InProgress: a dispatcher cycle is currently running (lock issue is open).
	InProgress
	// Blocked: all open issues require human intervention (conflict or human-review).
	Blocked
	// UpToDate: no open issues and fork is current with upstream.
	UpToDate
	// NeedsWork: one or more patch cycle phases have actionable work.
	NeedsWork
)

func (s State) String() string {
	return [...]string{
		"uninitialized", "in-progress", "blocked", "up-to-date", "needs-work",
	}[s]
}

// Actionable reports whether the dispatcher should enqueue a cycle.
func (s State) Actionable() bool { return s == NeedsWork }

// RepoInfo is the raw data gathered from GitHub to determine state and priority.
type RepoInfo struct {
	HasPatcherMD   bool
	Upstream       string    // "owner/repo" of the upstream (from PATCHER.md)
	LastPatched    string    // tag value from PATCHER.md, e.g. "v1.4.0"
	UpstreamLatest string    // latest release tag on the upstream repo
	LastPatchTime  time.Time // when the last *-patch release was published; zero if never
	HasLockIssue   bool
	OpenIssues     []IssueLabels
}

// IssueLabels holds the labels on a single open issue.
type IssueLabels struct {
	Labels []string
}

func (il IssueLabels) Has(label string) bool {
	for _, l := range il.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// Determine computes a repo's state from its gathered info.
func Determine(info RepoInfo) State {
	if !info.HasPatcherMD {
		return Uninitialized
	}
	if info.HasLockIssue {
		return InProgress
	}

	var hasActionable, hasBlocking bool
	for _, issue := range info.OpenIssues {
		if issue.Has("conflict") || issue.Has("human-review") {
			hasBlocking = true
		} else {
			hasActionable = true
		}
	}

	if hasActionable {
		return NeedsWork
	}
	if hasBlocking {
		return Blocked
	}
	if info.UpstreamLatest != "" && info.LastPatched != info.UpstreamLatest {
		return NeedsWork
	}
	return UpToDate
}

// Priority returns a staleness score: higher means process sooner.
// Repos never patched score highest; otherwise days since last patch release.
func Priority(info RepoInfo) float64 {
	if info.LastPatchTime.IsZero() {
		return 1e9
	}
	return time.Since(info.LastPatchTime).Hours() / 24
}
