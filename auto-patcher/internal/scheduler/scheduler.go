package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/auto-patcher/skills/internal/config"
	"github.com/auto-patcher/skills/internal/github"
	"github.com/auto-patcher/skills/internal/prompts"
	"github.com/auto-patcher/skills/internal/runner"
	"github.com/auto-patcher/skills/internal/state"
)

// Scheduler drives the worker pool. A background loop scans the org on a
// fixed interval and enqueues actionable repos in staleness order.
// Workers pace themselves with a configurable inter-job delay.
type Scheduler struct {
	cfg    *config.Config
	client *github.Client
	runner *runner.Runner
	queue  chan string
	active sync.Map // string → struct{}: repos currently being processed
}

func New(cfg *config.Config, client *github.Client, r *runner.Runner) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		client: client,
		runner: r,
		queue:  make(chan string, 256),
	}
}

// Run starts workers and the scan loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < s.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.worker(ctx, id)
		}(i)
	}

	s.scan(ctx)
	ticker := time.NewTicker(s.cfg.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(s.queue)
			wg.Wait()
			return nil
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Scheduler) scan(ctx context.Context) {
	slog.Info("scanning org", "org", s.cfg.Org)
	ranked, err := s.client.RankedRepos(ctx)
	if err != nil {
		slog.Error("scan failed", "err", err)
		return
	}
	enqueued := 0
	for _, entry := range ranked {
		if _, active := s.active.Load(entry.Repo); active {
			continue
		}
		select {
		case s.queue <- entry.Repo:
			enqueued++
		default:
			slog.Warn("queue full; will retry next scan", "repo", entry.Repo)
		}
	}
	slog.Info("scan complete", "actionable", len(ranked), "enqueued", enqueued)
}

func (s *Scheduler) worker(ctx context.Context, id int) {
	log := slog.With("worker", id)
	for {
		select {
		case <-ctx.Done():
			return
		case repo, ok := <-s.queue:
			if !ok {
				return
			}
			s.active.Store(repo, struct{}{})
			log.Info("starting cycle", "repo", repo)
			s.process(ctx, repo, log)
			s.active.Delete(repo)

			select {
			case <-ctx.Done():
				return
			case <-time.After(s.cfg.WorkerDelay):
			}
		}
	}
}

func (s *Scheduler) process(ctx context.Context, repo string, log *slog.Logger) {
	// Re-read repo info at process time — state may have changed since the scan.
	info, err := s.client.RepoInfo(ctx, repo)
	if err != nil {
		log.Error("failed to read repo info", "repo", repo, "err", err)
		return
	}
	if !state.Determine(info).Actionable() {
		log.Info("repo no longer actionable, skipping", "repo", repo)
		return
	}

	prompt, err := prompts.RenderCycle(prompts.Context{
		Repo:        repo,
		Upstream:    info.Upstream,
		LastPatched: info.LastPatched,
	})
	if err != nil {
		log.Error("failed to render prompt", "repo", repo, "err", err)
		return
	}

	if _, err := s.client.AcquireLock(ctx, repo); err != nil {
		log.Error("failed to acquire lock", "repo", repo, "err", err)
		return
	}
	defer s.client.ReleaseLock(ctx, repo)

	if err := s.runner.Run(ctx, runner.Job{Repo: repo, Prompt: prompt}); err != nil {
		log.Error("cycle failed", "repo", repo, "err", err)
		s.client.PostFailure(ctx, repo, err)
	} else {
		log.Info("cycle complete", "repo", repo)
	}
}
