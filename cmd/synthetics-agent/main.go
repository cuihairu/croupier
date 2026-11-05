package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/runner"
	"github.com/cuihairu/croupier/core/supervisable"
	"github.com/cuihairu/croupier/internal/telemetry/agentrec"
)

func main() {
	path := flag.String("config", "configs/synthetics-agent.yaml", "path to synthetics-agent yaml config")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Printf("synthetics-agent version %s (commit %s, built %s)\n", runner.Version, runner.GitCommit, runner.BuildTime)
		return
	}
	cfg, err := config.Load(*path)
	if err != nil {
		slog.Error("load config failed", "path", *path, "error", err)
		os.Exit(2)
	}

	r := runner.New(cfg, slog.Default())
	rec, err := agentrec.Build(context.Background(), cfg.Telemetry, slog.Default())
	if err != nil {
		// fail-closed：telemetry.enabled=true 但 no_otel 裁剪构建时不静默无导出。
		slog.Error("telemetry setup failed", "error", err)
		os.Exit(2)
	}
	r.SetRecorder(rec)

	os.Exit(supervisable.Main("synthetics-agent", func(ctx context.Context) error {
		defer func() { _ = rec.Shutdown(context.Background()) }()
		return r.Run(ctx)
	}, supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
