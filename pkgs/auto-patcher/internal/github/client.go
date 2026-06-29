package github

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"github.com/auto-patcher/skills/internal/config"
	"github.com/auto-patcher/skills/internal/state"
)

const lockIssueTitle = "[dispatcher] cycle in progress"

// Client wraps the GitHub API for all dispatcher operations.
type Client struct {
	cfg *config.Config
	gh  *gh.Client
}

func NewClient(cfg *config.Config) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.GitHubToken()})
	return &Client{
		cfg: cfg,
		gh:  gh.NewClient(oauth2.NewClient(context.Background(), ts)),
	}
}

// RankedRepo pairs a repo name with its computed state and priority.
type RankedRepo struct {
	Repo     string
	State    state.State
	Priority float64
}

// RankedRepos returns all actionable repos sorted by priority (most stale first).
func (c *Client) RankedRepos(ctx context.Context) ([]RankedRepo, error) {
	repos, err := c.listRepos(ctx)
	if err != nil {
		return nil, err
	}

	var ranked []RankedRepo
	for _, repo := range repos {
		info, err := c.RepoInfo(ctx, repo)
		if err != nil {
			slog.Error("failed to read repo info", "repo", repo, "err", err)
			continue
		}
		st := state.Determine(info)
		slog.Info("repo state", "repo", repo, "state", st)
		if st.Actionable() {
			ranked = append(ranked, RankedRepo{
				Repo:     repo,
				State:    st,
				Priority: state.Priority(info),
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Priority > ranked[j].Priority
	})
	return ranked, nil
}

// RepoInfo gathers state and priority data for a single repo.
func (c *Client) RepoInfo(ctx context.Context, repo string) (state.RepoInfo, error) {
	owner, name, _ := strings.Cut(repo, "/")
	info := state.RepoInfo{}

	content, _, resp, err := c.gh.Repositories.GetContents(ctx, owner, name, "PATCHER.md", nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return info, nil // Uninitialized
		}
		return info, fmt.Errorf("read PATCHER.md: %w", err)
	}
	info.HasPatcherMD = true
	body, err := content.GetContent()
	if err != nil {
		return info, fmt.Errorf("decode PATCHER.md: %w", err)
	}
	upstream, lastPatched := parsePatcherMD(body)
	info.Upstream = upstream
	info.LastPatched = lastPatched

	if upstream != "" {
		if upOwner, upName, ok := strings.Cut(upstream, "/"); ok {
			if rel, _, err := c.gh.Repositories.GetLatestRelease(ctx, upOwner, upName); err == nil {
				info.UpstreamLatest = rel.GetTagName()
			} else {
				slog.Warn("could not fetch upstream latest release", "upstream", upstream, "err", err)
			}
		}
	}

	releases, _, _ := c.gh.Repositories.ListReleases(ctx, owner, name, &gh.ListOptions{PerPage: 10})
	for _, r := range releases {
		if strings.HasSuffix(r.GetTagName(), "-patch") {
			info.LastPatchTime = r.GetPublishedAt().Time
			break
		}
	}

	issues, _, err := c.gh.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{
		State: "open", ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		return info, fmt.Errorf("list issues: %w", err)
	}
	for _, issue := range issues {
		if issue.GetTitle() == lockIssueTitle {
			info.HasLockIssue = true
			continue
		}
		var labels []string
		for _, l := range issue.Labels {
			labels = append(labels, l.GetName())
		}
		info.OpenIssues = append(info.OpenIssues, state.IssueLabels{Labels: labels})
	}

	return info, nil
}

// AcquireLock creates the dispatcher lock issue.
func (c *Client) AcquireLock(ctx context.Context, repo string) (bool, error) {
	owner, name, _ := strings.Cut(repo, "/")
	_, _, err := c.gh.Issues.Create(ctx, owner, name, &gh.IssueRequest{
		Title:  gh.String(lockIssueTitle),
		Labels: &[]string{"in-progress"},
	})
	if err != nil {
		return false, fmt.Errorf("create lock issue: %w", err)
	}
	return true, nil
}

// ReleaseLock closes the dispatcher lock issue.
func (c *Client) ReleaseLock(ctx context.Context, repo string) {
	owner, name, _ := strings.Cut(repo, "/")
	issues, _, err := c.gh.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{
		State: "open", ListOptions: gh.ListOptions{PerPage: 20},
	})
	if err != nil {
		slog.Error("ReleaseLock: list issues", "repo", repo, "err", err)
		return
	}
	closed := "closed"
	for _, issue := range issues {
		if issue.GetTitle() == lockIssueTitle {
			_, _, err = c.gh.Issues.Edit(ctx, owner, name, issue.GetNumber(),
				&gh.IssueRequest{State: &closed})
			if err != nil {
				slog.Error("ReleaseLock: close issue", "repo", repo, "err", err)
			}
			return
		}
	}
}

// PostFailure opens a human-review issue describing the cycle failure.
func (c *Client) PostFailure(ctx context.Context, repo string, failure error) {
	owner, name, _ := strings.Cut(repo, "/")
	body := fmt.Sprintf(
		"The dispatcher encountered an error running a patch cycle.\n\n"+
			"```\n%s\n```\n\n"+
			"Investigate, then remove the `human-review` label to re-enable automated processing.",
		failure,
	)
	_, _, err := c.gh.Issues.Create(ctx, owner, name, &gh.IssueRequest{
		Title:  gh.String("[dispatcher] cycle failed"),
		Body:   gh.String(body),
		Labels: &[]string{"human-review"},
	})
	if err != nil {
		slog.Error("PostFailure: create issue", "repo", repo, "err", err)
	}
}

func (c *Client) listRepos(ctx context.Context) ([]string, error) {
	excluded := make(map[string]bool, len(c.cfg.Exclude))
	for _, e := range c.cfg.Exclude {
		excluded[e] = true
	}
	var repos []string
	opts := &gh.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		page, resp, err := c.gh.Repositories.ListByOrg(ctx, c.cfg.Org, opts)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}
		for _, r := range page {
			if !excluded[r.GetName()] {
				repos = append(repos, c.cfg.Org+"/"+r.GetName())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return repos, nil
}

func parsePatcherMD(content string) (upstream, lastPatched string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "upstream:"); ok {
			upstream = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "last_patched:"); ok {
			lastPatched = strings.TrimSpace(v)
		}
	}
	return
}
