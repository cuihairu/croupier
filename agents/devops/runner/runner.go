// Package runner devops-agent 装配：注册上线/心跳（core/register，payload
// 带 game_id+env 标签，role=devops）+ execlog 审计真值落盘 + 会话建立链
// （Broker）+ 会话数据面（Hub：SSH 三向数据流/窗口调整/背压/断线清理/
// asciicast 回放存档）。`devops.enabled=false` 时仅注册上线（面板可见，
// 会话面关闭，agent-core 简档 §7.4）。
//
// 白名单下发：config_versions 键 devops.allowlist（namespace=ops）经 wire
// 拉取（PullDevopsAllowlist）+ core/configsync 热生效（文件配置为兜底，
// 中心版本生效后整体覆盖；面板发布预检与 agent apply 共用同一校验真值）。
//
// 边界（诚实）：面板→server→agent 的会话建立下行通道与流式中继未接
// （agent 侧会话面经 Hub.Attach 暴露接线点，中继随下行控制批）；RDP/VNC
// （guacd 通道）另批——allowlist 声明了 rdp/vnc 时 Open 显式拒绝。
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/agents/devops/session"
	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/execlog"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// devopsAllowlistConfigKey config_versions 里的白名单键（server 拉取侧
// internal/server 同名常量，两侧键一致是 wire 契约）。
const devopsAllowlistConfigKey = "devops.allowlist"

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
	hub    *session.Hub

	// conn 最近一次 Dial 成产出的上行连接（白名单拉取跟随重连换新）。
	conn atomic.Pointer[controlconn.Conn]
	// fetchAllowlist 拉取 seam（测试注入；生产经 conn.PullDevopsAllowlist）。
	fetchAllowlist func(ctx context.Context) (*configsync.Versioned[[]byte], error)
	// applyAllowlist 热生效 seam（测试断言；生产为校验+Swap+日志）。
	applyAllowlist func(ctx context.Context, payload []byte) error
}

// New 装配 Runner。cfg 须先经 config.Load 校验。
func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	r := &Runner{cfg: cfg, log: log}
	r.fetchAllowlist = func(ctx context.Context) (*configsync.Versioned[[]byte], error) {
		c := r.conn.Load()
		if c == nil || !c.Connected() {
			return nil, errors.New("control conn not ready")
		}
		resp, err := c.PullDevopsAllowlist(ctx, &opsv1.DevopsAllowlistConfigRequest{AgentId: r.cfg.Agent.ID})
		if err != nil {
			return nil, err
		}
		// 空版本 = 服务端未发布过白名单：返回 nil 载荷，Puller 保持现状。
		if strings.TrimSpace(resp.GetVersion()) == "" {
			return nil, nil
		}
		return &configsync.Versioned[[]byte]{Version: resp.GetVersion(), Payload: []byte(resp.GetAllowlistJson())}, nil
	}
	return r
}

// Broker 会话建立链句柄（测试观察）。
func (r *Runner) Broker() *session.Broker { return r.broker }

// Hub 会话数据面句柄（下行中继批的接线点；测试观察）。
func (r *Runner) Hub() *session.Hub { return r.hub }

// Run 是 supervisable.RunFunc：注册上线 + 审计/会话面装配，ctx 取消即收尾。
func (r *Runner) Run(ctx context.Context) error {
	reg := register.New(register.Config{
		AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			c, err := controlconn.Dial(r.cfg.Agent.ServerAddr, tlsFromCfg(r.cfg))
			if err == nil {
				r.conn.Store(c)
			}
			return c, err
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

	// SSH 真实通道（构造期装载凭据与指纹策略；失败即不起会话面）。
	target, err := session.NewSSHChannelTarget(sshConfigFromCfg(r.cfg))
	if err != nil {
		return fmt.Errorf("devops ssh channel: %w", err)
	}
	if r.cfg.Devops.SSH.InsecureHostKey {
		r.log.Warn("devops ssh host key verification DISABLED (insecureHostKey=true) — test use only")
	}

	allow := session.NewAllowlist(r.cfg.Devops.Allowlist)
	r.broker = session.NewBroker(
		allow,
		&session.ExecAudit{Gate: logger.Gate(), Log: logger},
		target,
		recorder,
		"devops", r.cfg.Agent.ID, r.cfg.Agent.GameID, r.cfg.Agent.Env,
	)
	castDir := filepath.Join(r.cfg.SessionDir(), "casts")
	r.hub = session.NewHub(r.broker, recorder, castDir, session.HubConfig{}, r.log)
	defer func() { _ = r.hub.Close() }()

	// 白名单 configsync 下发（capture gates/probe policy 同款通道语义：
	// 周期拉取 → 版本比对 → 校验 → 整体热换；坏版本 keep last-good）。
	r.newAllowlistPuller().Start(ctx)

	r.log.Info("devops session plane enabled",
		"allowlist", allow.Len(), "channel", target.Name(),
		"castDir", castDir, "user", r.cfg.Devops.SSH.User)

	<-ctx.Done()
	return nil
}

// newAllowlistPuller 白名单下行拉取器（测试可同步 PullOnce，不启 goroutine）。
func (r *Runner) newAllowlistPuller() *configsync.Puller[[]byte] {
	if r.applyAllowlist == nil {
		r.applyAllowlist = r.applyAllowlistCenter
	}
	return configsync.New[[]byte](configsync.DefaultInterval, r.fetchAllowlist, r.applyAllowlist,
		configsync.WithOnApply[[]byte](func(version string) {
			r.log.Info("devops allowlist version applied", "version", version)
		}))
}

// applyAllowlistCenter 校验后整体热换白名单（面板发布预检与 agent apply
// 同一校验真值；任一条非法即整体拒绝，保持 last-good）。
func (r *Runner) applyAllowlistCenter(_ context.Context, payload []byte) error {
	entries, err := config.ParseAllowlistJSON(payload)
	if err != nil {
		return err
	}
	if err := config.ValidateAllowlist(entries); err != nil {
		return err
	}
	if r.broker == nil {
		return errors.New("broker not wired")
	}
	prev, cur := r.broker.SwapAllowlist(entries)
	r.log.Info("devops allowlist swapped", "previous", prev, "current", cur)
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

// sshConfigFromCfg 配置面 → SSH 通道装配参数。
func sshConfigFromCfg(cfg config.Config) session.SSHConfig {
	s := cfg.Devops.SSH
	return session.SSHConfig{
		User:            s.User,
		PrivateKeyFile:  s.PrivateKeyFile,
		PasswordFile:    s.PasswordFile,
		KnownHostsFile:  s.KnownHostsFile,
		InsecureHostKey: s.InsecureHostKey,
		ConnectTimeout:  time.Duration(s.ConnectTimeoutMs) * time.Millisecond,
	}
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
