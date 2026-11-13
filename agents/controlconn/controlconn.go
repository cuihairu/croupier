// Package controlconn core 系 agent 共用上行控制连接（core/register.Conn 适配
// core/transport/tcp.Client）。语义对齐 internal/app/agent 的 tcpControlClient
// （Register/Heartbeat 走 agentv1 wire），但依赖面收在 agents/*→core/*→proto
// 不变量内：不 import internal/。
package controlconn

import (
	"context"
	"fmt"
	"sync"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// TLS 上行通道 TLS 配置（对齐 configs/agent.yaml outboundTLS 块语义）。
// nil / Enabled=false → Insecure 裸连。
type TLS struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	CertFile           string `yaml:"certFile" json:"certFile"`
	KeyFile            string `yaml:"keyFile" json:"keyFile"`
	CAFile             string `yaml:"caFile" json:"caFile"`
	ServerName         string `yaml:"serverName" json:"serverName"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
}

// Conn 上行控制连接，实现 core/register.Conn。
type Conn struct {
	client *tcp.Client
}

// Dial 建立上行控制连接（未注册；注册由 core/register.Client 驱动）。
func Dial(addr string, tlsCfg *TLS) (*Conn, error) {
	cfg := &tcp.Config{
		Address:        normalizeAddr(addr),
		Insecure:       tlsCfg == nil || !tlsCfg.Enabled,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    10 * time.Second,
	}
	if tlsCfg != nil && tlsCfg.Enabled {
		cfg.CertFile = tlsCfg.CertFile
		cfg.KeyFile = tlsCfg.KeyFile
		cfg.CAFile = tlsCfg.CAFile
		cfg.ServerName = tlsCfg.ServerName
		cfg.InsecureSkipVerify = tlsCfg.InsecureSkipVerify
	}
	client, err := tcp.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("dial control %s: %w", addr, err)
	}
	return &Conn{client: client}, nil
}

// Connected 连接是否存活。
func (c *Conn) Connected() bool { return c != nil && c.client != nil && !c.client.IsClosed() }

// Close 关闭连接（幂等）。
func (c *Conn) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Register 上线注册。
func (c *Conn) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	resp := &agentv1.RegisterResponse{}
	if err := c.call(ctx, protocol.MsgRegisterRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Heartbeat 存活心跳。
func (c *Conn) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	resp := &agentv1.HeartbeatResponse{}
	if err := c.call(ctx, protocol.MsgHeartbeatRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendAlertReport 上报 capture 规则命中告警（服务端幂等落 alerts 表）。
func (c *Conn) SendAlertReport(ctx context.Context, req *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
	resp := &opsv1.CaptureRuleHitAck{}
	if err := c.call(ctx, protocol.MsgAlertEventRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendProbeEvent sends a probe result through the shared Agent session's existing AlertEvent wire.
func (c *Conn) SendProbeEvent(ctx context.Context, req *opsv1.AlertEvent) error {
	return c.notify(ctx, protocol.MsgAlertEvent, req)
}

// PullProbePolicy pulls the latest game/env-scoped probe sampling policy.
func (c *Conn) PullProbePolicy(ctx context.Context, req *opsv1.ProbePolicyConfigRequest) (*opsv1.ProbePolicyConfigResponse, error) {
	resp := &opsv1.ProbePolicyConfigResponse{}
	if err := c.call(ctx, protocol.MsgProbePolicyRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// PullSyntheticsTargets pulls the latest game/env-scoped synthetics
// target list (config_versions synthetics.targets; configsync pull).
func (c *Conn) PullSyntheticsTargets(ctx context.Context, req *opsv1.SyntheticsTargetsConfigRequest) (*opsv1.SyntheticsTargetsConfigResponse, error) {
	resp := &opsv1.SyntheticsTargetsConfigResponse{}
	if err := c.call(ctx, protocol.MsgSyntheticsTargetsConfigRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// PullCaptureGates 拉取 capture 闸规则下行配置（服务端读 config_versions
// 的 capture.gates，version 空串 = 未发布，agent 保持现状）。
func (c *Conn) PullCaptureGates(ctx context.Context, req *opsv1.CaptureGatesConfigRequest) (*opsv1.CaptureGatesConfigResponse, error) {
	resp := &opsv1.CaptureGatesConfigResponse{}
	if err := c.call(ctx, protocol.MsgCaptureConfigRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// PullDevopsAllowlist 拉取 devops 会话白名单下行配置（服务端读 config_versions
// 的 devops.allowlist，version 空串 = 未发布，agent 保持现状）。
func (c *Conn) PullDevopsAllowlist(ctx context.Context, req *opsv1.DevopsAllowlistConfigRequest) (*opsv1.DevopsAllowlistConfigResponse, error) {
	resp := &opsv1.DevopsAllowlistConfigResponse{}
	if err := c.call(ctx, protocol.MsgDevopsAllowlistConfigRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Conn) notify(ctx context.Context, msgID uint32, req proto.Message) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("control conn not initialized")
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return c.client.Send(ctx, msgID, data)
}

func (c *Conn) call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("control conn not initialized")
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	_, respData, err := c.client.Call(ctx, msgID, data)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// normalizeAddr 剥 tcp:// 前缀（对齐 sidecar 配置宽容度）。
func normalizeAddr(addr string) string {
	for _, p := range []string{"tcp://", "tcp:"} {
		if len(addr) >= len(p) && addr[:len(p)] == p {
			addr = addr[len(p):]
		}
	}
	return addr
}

// MuxSession 双向控制连接（AGENT-002 devops 会话中继面）：MuxConn 之上的
// register.Conn 适配。读循环后台运行——server 下行的会话请求经 handler
// 处理并回帧，agent 上行经 Call（拉取/注册/心跳）与 Send（会话事件流）。
type MuxSession struct {
	mux    *tcp.MuxConn
	cancel context.CancelFunc
	// onClosed 连接读循环退出（断连）回调，恰发一次；nil = 忽略。devops
	// runner 用它收束会话面（控制面丢失 = 会话面关闭，堡垒机安全缺省）。
	onClosedMu sync.Mutex
	onClosed   func()
	closedOnce sync.Once
}

// DialMux 建立双向控制连接。handler 处理 server 下行请求/事件（可为 nil：
// 下行请求将被协议错误拒断——只上行拉取的 agent 用 Dial 即可）。
func DialMux(addr string, tlsCfg *TLS, handler transportcore.Handler) (*MuxSession, error) {
	cfg := &tcp.Config{
		Address:        normalizeAddr(addr),
		Insecure:       tlsCfg == nil || !tlsCfg.Enabled,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    10 * time.Second,
	}
	if tlsCfg != nil && tlsCfg.Enabled {
		cfg.CertFile = tlsCfg.CertFile
		cfg.KeyFile = tlsCfg.KeyFile
		cfg.CAFile = tlsCfg.CAFile
		cfg.ServerName = tlsCfg.ServerName
		cfg.InsecureSkipVerify = tlsCfg.InsecureSkipVerify
	}
	conn, err := tcp.Dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("dial control %s: %w", addr, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	mux := tcp.NewMuxConn(conn, cfg, handler)
	s := &MuxSession{mux: mux, cancel: cancel}
	go func() {
		_ = mux.Run(ctx)
		s.closedOnce.Do(func() {
			s.onClosedMu.Lock()
			fn := s.onClosed
			s.onClosedMu.Unlock()
			if fn != nil {
				fn()
			}
		})
	}()
	return s, nil
}

// SetOnClosed 装配断连回调（读循环退出时恰发一次；须在 DialMux 后尽早调用，
// 丢掉极早期断连属可接受竞态——register 重连链随后重拨）。
func (c *MuxSession) SetOnClosed(fn func()) {
	c.onClosedMu.Lock()
	c.onClosed = fn
	c.onClosedMu.Unlock()
}

// Connected 连接是否存活。
func (c *MuxSession) Connected() bool { return c != nil && c.mux != nil && !c.mux.IsClosed() }

// Close 关闭连接（幂等）。
func (c *MuxSession) Close() error {
	if c == nil || c.mux == nil {
		return nil
	}
	c.cancel()
	return c.mux.Close()
}

// Register 上线注册。
func (c *MuxSession) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	resp := &agentv1.RegisterResponse{}
	if err := c.call(ctx, protocol.MsgRegisterRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Heartbeat 存活心跳。
func (c *MuxSession) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	resp := &agentv1.HeartbeatResponse{}
	if err := c.call(ctx, protocol.MsgHeartbeatRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Call 下行请求的调用面（devops 白名单拉取同 wire；会话事件经 Send）。
func (c *MuxSession) Call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	return c.call(ctx, msgID, req, resp)
}

// Send 上行单发事件帧（devops 会话输出/结束事件流）。
func (c *MuxSession) Send(ctx context.Context, msgID uint32, msg proto.Message) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("control conn not initialized")
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return c.mux.Send(ctx, msgID, data)
}

func (c *MuxSession) call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("control conn not initialized")
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	_, respData, err := c.mux.Call(ctx, msgID, data)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}
