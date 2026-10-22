// Package runner devops-agent 装配：注册上线/心跳（core/register，payload
// 带 game_id+env 标签，role=devops）+ execlog 审计真值落盘 + 会话建立链
// （Broker）装配。`devops.enabled=false` 时仅注册上线（面板可见，会话面
// 关闭，agent-core 简档 §7.4）。
//
// 骨架边界（诚实）：面板→agent 的会话建立下行通道未接（Broker 经单测验证，
// 接线随下行控制批）；协议 Provider 为 noop（guacd/SSH 通道另批）；白名单
// configsync 下发另批（文件配置起步）。
package runner

import (
	"context"
	"log/slog"
	"os"
	"runtime"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/agents/devops/session"
	"github.com/cuihairu/croupier/core/execlog"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// Version 注册携带的 agent 版本（可经 ldflags 覆盖）。
var Version = "dev"

// GitCommit/BuildTime 构建元信息（Makefile agent-devops target 注入，注册携带）。
var (
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Runner 是 supervisable.RunFunc 的载体。
type Runner struct {
	cfg    config.Config
	log    *slog.Logger
	broker *session.Broker
}

// New 装配 Runner。cfg 须先经 config.Load 校验。
func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{cfg: cfg, log: log}
}

// Broker 供下行会话通道批接线（骨架期暴露给测试）。
func (r *Runner) Broker() *session.Broker { return r.broker }

// Run 是 supervisable.RunFunc：注册上线 + 审计/会话面装配，ctx 取消即收尾。
func (r *Runner) Run(ctx context.Context) error {
	reg := register.New(register.Config{
		AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			return controlconn.Dial(r.cfg.Agent.ServerAddr, tlsFromCfg(r.cfg))
		},
		Payload: r.buildPayload,
		Logger:  r.log,
		OnConnected: func() {
			r.log.Info("registered upstream", "agent", r.cfg.Agent.ID, "gameId", r.cfg.Agent.GameID, "env", r.cfg.Agent.Env)
		},
	})
	reg.Start(ctx)
	defer reg.Stop()

	if !r.cfg.Devops.Enabled {
		r.log.Info("devops sessions disabled, registration only", "agent", r.cfg.Agent.ID)
		<-ctx.Done()
		return nil
	}

	// 审计真值落盘（audit-first 前提）：打不开 = 无审计通道，会话面不得
	// 服务——返回错误退出交监管，不降级为「无审计放行」。
	writer, err := execlog.OpenFileWriter(r.cfg.ExeclogDir(), "exec.log", execlog.WriterOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()
	logger := execlog.New(writer)

	recorder, err := session.OpenRecorder(r.cfg.SessionDir())
	if err != nil {
		return err
	}
	defer func() { _ = recorder.Close() }()

	r.broker = session.NewBroker(
		session.NewAllowlist(r.cfg.Devops.Allowlist),
		&session.ExecAudit{Gate: logger.Gate(), Log: logger},
		session.NewNoopTarget(),
		recorder,
		"devops", r.cfg.Agent.ID, r.cfg.Agent.GameID, r.cfg.Agent.Env,
	)
	r.log.Info("devops session plane enabled", "allowlist", len(r.cfg.Devops.Allowlist),
		"channel", "noop (protocol channels land in a later batch)")

	<-ctx.Done()
	return nil
}

// buildPayload 组装 RegisterRequest（role=devops；无函数面——core 系 agent
// 没有函数注册能力是定位而非缺陷）。
func (r *Runner) buildPayload(context.Context) (*agentv1.RegisterRequest, error) {
	host, _ := os.Hostname()
	return &agentv1.RegisterRequest{
		AgentId: r.cfg.Agent.ID,
		Version: Version,
		GameId:  r.cfg.Agent.GameID,
		Env:     r.cfg.Agent.Env,
		Labels: map[string]string{
			"role":      "devops",
			"hostname":  host,
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"gitCommit": GitCommit,
			"buildTime": BuildTime,
		},
	}, nil
}

func tlsFromCfg(cfg config.Config) *controlconn.TLS {
	t := cfg.OutboundTLS
	if !t.Enabled {
		return nil
	}
	return &controlconn.TLS{
		Enabled:            true,
		CertFile:           t.CertFile,
		KeyFile:            t.KeyFile,
		CAFile:             t.CAFile,
		ServerName:         t.ServerName,
		InsecureSkipVerify: t.InsecureSkipVerify,
	}
}
