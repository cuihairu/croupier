// devops-agent：内网堡垒机式远程接入代理（agent-core 简档 §7 骨架）。
//
// 装配：config.Load（devops.enabled 缺省关，注册上线不受影响）→ runner
// （注册/心跳 + execlog 审计真值 + 会话建立链 Broker）→ supervisable.Main
// （信号纪律/心跳打点/退出码契约）。协议通道（guacd/SSH）另批。
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/agents/devops/runner"
	"github.com/cuihairu/croupier/core/supervisable"
)

func main() {
	configPath := flag.String("config", "configs/devops-agent.yaml", "path to devops-agent yaml config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		slog.Info("devops-agent", "version", runner.Version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		// exit 2 = 配置错误（与运行异常 1 区分，监管日志可辨）。
		slog.Error("load config failed", "path", *configPath, "error", err)
		os.Exit(2)
	}

	os.Exit(supervisable.Main("devops-agent", runner.New(cfg, slog.Default()).Run,
		supervisable.WithHeartbeat(cfg.Agent.HeartbeatFile, 0)))
}
