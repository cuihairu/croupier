// Package register 注册上线 + 心跳存活 + 重连（上收自 sidecar-agent 的
// upstream 注册链，agent-core K2）。core 只管「连上→注册→心跳→断线重连」
// 通用循环；注册 payload 组装（函数/进程清单等业务面）经 PayloadProvider
// 由业务侧供给，传输构造经 Dialer 注入——core 不 import internal/，协议
// wire 类型唯一来源仍是 proto（agentv1）。
package register

import (
	"context"
	"log/slog"
	"sync"
	"time"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// Conn 上游控制连接的最小面（注册/心跳/存活/关闭）；业务侧扩展发送
// （任务事件/指标等）经 Client.Conn() 取回同一连接自行调用。
type Conn interface {
	Connected() bool
	Close() error
	Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)
	Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)
}

// Dialer 建立一条新上游连接（内部不注册；注册由 Client 统一驱动）。
type Dialer func(ctx context.Context) (Conn, error)

// PayloadProvider 每次注册（含重连后的重注册）组装 RegisterRequest。
type PayloadProvider func(ctx context.Context) (*agentv1.RegisterRequest, error)

// Config 客户端配置；零值字段用默认（对齐 sidecar 现状）。
type Config struct {
	AgentID string
	Dial    Dialer
	Payload PayloadProvider

	HeartbeatInterval    time.Duration // 默认 3s
	HeartbeatTimeout     time.Duration // 单次心跳调用超时，默认 3s
	RequestTimeout       time.Duration // 单次注册调用超时，默认 10s
	MaxHeartbeatFailures int           // 连续心跳失败达阈值重连，默认 2

	// NewDialBackoff/NewSyncBackoff 退避注入（测试用快档；默认指数 5s→60s
	// ×1.5 / 200ms→2s，见 core/backoff）。
	NewDialBackoff func() corebackoff.BackOff
	NewSyncBackoff func() corebackoff.BackOff

	OnConnected func() // 每次注册成功后触发（与 sidecar 现状一致，可能重复触发）
	Logger      *slog.Logger
}

// Client 上游注册客户端：Start 后台自愈（断线重连+心跳失败重连+恢复后
// 重注册），直到 ctx 取消或 Stop。
type Client struct {
	cfg Config
	log *slog.Logger

	mu            sync.Mutex
	conn          Conn
	ownerInstance string // 最近注册响应中的集群实例 ID（心跳携带，三方对账）
}
