package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/runner"
	"github.com/cuihairu/croupier/core/supervisable"
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
	os.Exit(supervisable.Main("synthetics-agent", runner.New(cfg, slog.Default()).Run, supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
