// capture-agent：游戏库变更捕获（CDC）agent（#66 系 capture C1 骨架）。
//
// 装配：config.Load（capture.enabled 缺省关）→ runner（注册上线/心跳 +
// 事件流消费 + 位点落盘 + Readiness 探库）→ supervisable.Main（信号纪律/
// 心跳打点/退出码契约——被 supervisor 托管即出厂合格被监管对象）。
// 日志走 slog→stderr（面板日志入口复用 supervisor 日志下载链，设计 §4）。
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

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/runner"
	"github.com/cuihairu/croupier/core/supervisable"
	"github.com/cuihairu/croupier/internal/telemetry/agentrec"
)

func main() {
	configPath := flag.String("config", "configs/capture-agent.yaml", "path to capture-agent yaml config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		// 版本输出走 stdout（安装脚本自校验捕 stdout；slog 默认 stderr）
		fmt.Printf("capture-agent version %s (commit %s, built %s)\n", runner.Version, runner.GitCommit, runner.BuildTime)
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

	os.Exit(supervisable.Main("capture-agent", func(ctx context.Context) error {
		defer func() { _ = rec.Shutdown(context.Background()) }()
		return r.Run(ctx)
	}, supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
