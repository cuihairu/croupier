package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	analyticsapi "github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/svc"
	redis "github.com/redis/go-redis/v9"
)

// startPipelineMonitor wires the analytics pipeline health monitor into the
// server process (short-term data monitoring; see
// internal/api/analytics/pipeline_monitor.go). Data-side incidents
// (ClickHouse down / event stream stall / volume drop / dead-letter and MQ
// backlog) land in the ops alert center. No-ops when the alert model is
// missing; CH/Redis checks degrade individually when unconfigured.
func startPipelineMonitor(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx == nil || svcCtx.AlertModel == nil {
		return
	}
	var deadCounter analyticsapi.DeadLetterCounter
	if url := strings.TrimSpace(os.Getenv("REDIS_URL")); url != "" {
		if opt, err := redis.ParseURL(url); err == nil {
			deadCounter = redis.NewClient(opt)
		} else {
			slog.Warn("pipeline monitor: parse REDIS_URL failed; dead letter/MQ backlog checks disabled", "err", err)
		}
	}
	monitor := analyticsapi.NewPipelineMonitor(svcCtx.AlertModel, deadCounter)
	go monitor.Run(ctx)
	slog.Info("pipeline monitor started",
		"clickhouse", os.Getenv("CLICKHOUSE_DSN") != "",
		"redis", deadCounter != nil)
}
