// devops-agent：内网堡垒机式远程接入代理（agent-core 简档 §7 骨架）。
//
// 装配：config.Load（devops.enabled 缺省关，注册上线不受影响）→ runner
// （注册/心跳 + execlog 审计真值 + 会话建立链 Broker）→ supervisable.Main
// （信号纪律/心跳打点/退出码契约）。协议通道（guacd/SSH）另批。
// OTel 观测面（OBS-AGENT-001）：telemetry.enabled=false（缺省）不建
// exporter/周期导出任务（runner 走 NoopRecorder 零开销）；true 时经
// agentrec.Build 装配 OTLP 导出并在 run 收尾 Shutdown 冲刷。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/agents/devops/runner"
	"github.com/cuihairu/croupier/core/supervisable"
	"github.com/cuihairu/croupier/internal/telemetry/agentrec"
)

func main() {
	configPath := flag.String("config", "configs/devops-agent.yaml", "path to devops-agent yaml config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		// 版本输出走 stdout（安装脚本自校验捕 stdout；slog 默认 stderr）
		fmt.Printf("devops-agent version %s (commit %s, built %s)\n", runner.Version, runner.GitCommit, runner.BuildTime)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		// exit 2 = 配置错误（与运行异常 1 区分，监管日志可辨）。
		slog.Error("load config failed", "path", *configPath, "error", err)
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

	os.Exit(supervisable.Main("devops-agent", func(ctx context.Context) error {
		defer func() { _ = rec.Shutdown(context.Background()) }()
		return r.Run(ctx)
	}, supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
