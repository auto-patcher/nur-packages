package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/auto-patcher/skills/internal/config"
	"github.com/auto-patcher/skills/internal/github"
	"github.com/auto-patcher/skills/internal/runner"
	"github.com/auto-patcher/skills/internal/scheduler"
)

func main() {
	configPath := flag.String("config", envOr("DISPATCHER_CONFIG", "config.yaml"), "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	client := github.NewClient(cfg)
	r := runner.New(cfg)
	sched := scheduler.New(cfg, client, r)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("dispatcher starting",
		"org", cfg.Org,
		"workers", cfg.Workers,
		"scan_interval", cfg.ScanInterval,
		"worker_delay", cfg.WorkerDelay,
	)

	if err := sched.Run(ctx); err != nil {
		slog.Error("dispatcher exited", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
