// Package runner downloads a fork repository snapshot via the GitHub API and
// invokes claude non-interactively with a pre-rendered prompt. No git or gh
// CLI dependency is required.
package runner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gogithub "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"github.com/auto-patcher/skills/internal/config"
)

// Job describes a single patch cycle run.
type Job struct {
	Repo   string // "owner/repo" — downloaded into a temp directory
	Prompt string // fully rendered prompt, passed to claude via stdin
}

// Runner downloads a repo snapshot and runs a Claude cycle against it.
type Runner struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run downloads the fork into a temp directory, passes the rendered prompt to
// claude via stdin, and removes the temp directory when done.
func (r *Runner) Run(ctx context.Context, job Job) error {
	workDir, err := os.MkdirTemp("", "autopatcher-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := r.download(ctx, job.Repo, workDir); err != nil {
		return err
	}
	return r.runClaude(ctx, job.Prompt, workDir)
}

// download fetches the default-branch tarball via the GitHub API and extracts
// it into dir, stripping the top-level directory that GitHub adds.
func (r *Runner) download(ctx context.Context, repo, dir string) error {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return fmt.Errorf("invalid repo %q: expected owner/repo", repo)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: r.cfg.GitHubToken()})
	hc := oauth2.NewClient(ctx, ts)
	gh := gogithub.NewClient(hc)

	// GetArchiveLink with followRedirects=true returns the final S3 URL.
	archiveURL, _, err := gh.Repositories.GetArchiveLink(
		ctx, owner, name, gogithub.Tarball,
		&gogithub.RepositoryContentGetOptions{},
		1, // max redirects
	)
	if err != nil {
		return fmt.Errorf("get archive link %s: %w", repo, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL.String(), nil)
	if err != nil {
		return fmt.Errorf("build archive request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("download archive %s: %w", repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download archive %s: HTTP %d", repo, resp.StatusCode)
	}

	return extractTarGz(resp.Body, dir)
}

// extractTarGz extracts a gzipped tar archive into dir, stripping the single
// top-level directory that GitHub includes in all archive downloads.
func extractTarGz(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var stripPrefix string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		// The first entry is always the root directory (e.g. "owner-repo-abc123/").
		// Record it so we can strip it from every subsequent path.
		if stripPrefix == "" {
			stripPrefix = strings.SplitN(hdr.Name, "/", 2)[0] + "/"
		}

		rel := strings.TrimPrefix(hdr.Name, stripPrefix)
		if rel == "" {
			continue
		}

		// Sanitize: reject paths that escape the target directory.
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if !strings.HasPrefix(target+string(os.PathSeparator), dir+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q would escape target directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}

func (r *Runner) runClaude(ctx context.Context, prompt, dir string) error {
	// TODO: update flag/invocation once the auto-patcher CC fork's
	// non-interactive API is finalised.
	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY="+r.cfg.AnthropicKey(),
		"GITHUB_TOKEN="+r.cfg.GitHubToken(),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude: %w\noutput:\n%s", err, out)
	}
	return nil
}
