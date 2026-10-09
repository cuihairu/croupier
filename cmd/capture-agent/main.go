// capture-agent：游戏库变更捕获（CDC）agent（#66 系 capture C1 骨架）。
//
// 装配：config.Load（capture.enabled 缺省关）→ runner（注册上线/心跳 +
// 事件流消费 + 位点落盘 + Readiness 探库）→ supervisable.Main（信号纪律/
// 心跳打点/退出码契约——被 supervisor 托管即出厂合格被监管对象）。
// 日志走 slog→stderr（面板日志入口复用 supervisor 日志下载链，设计 §4）。
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/runner"
	"github.com/cuihairu/croupier/core/supervisable"
)

func main() {
	configPath := flag.String("config", "configs/capture-agent.yaml", "path to capture-agent yaml config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		slog.Info("capture-agent", "version", runner.Version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		// exit 2 = 配置错误（与运行异常 1 区分，监管日志可辨）。
		slog.Error("load config failed", "path", *configPath, "error", err)
		os.Exit(2)
	}

	os.Exit(supervisable.Main("capture-agent", runner.New(cfg, slog.Default()).Run,
		supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
