package agent

import (
	"context"
	"log/slog"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"

	"google.golang.org/protobuf/proto"
)

// ProviderKeepalive 是 provider session 的传输层保活探针：
// session 空闲超过 interval 时，agent 主动向 provider 发一次
// ProviderHeartbeatRequest（pong 必须经过 SDK 事件循环回出）。
// pong 超时 = 连接断开 或 SDK 事件循环卡死 → Close + 从池移除，
// 调用路由（callProvider）不再选到"进程活着但处理不动"的 provider。
//
// 设计原则：不新增状态机制——探测帧复用既有 ProviderHeartbeat 消息族，
// 摘除复用既有 ProviderSessionStore.Remove；间隔可配置，默认 5s。
type ProviderKeepalive struct {
	store    *ProviderSessionStore
	interval time.Duration
	logger   *slog.Logger
}

// NewProviderKeepalive 创建探针。interval <= 0 默认 5s（SDK 业务心跳
// 周期 30s，探测显著短于业务心跳才能在 prune 之前发现卡死）。
func NewProviderKeepalive(store *ProviderSessionStore, interval time.Duration, logger *slog.Logger) *ProviderKeepalive {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ProviderKeepalive{store: store, interval: interval, logger: logger}
}

// Run 阻塞运行探测循环（ctx 退出即停）。
func (k *ProviderKeepalive) Run(ctx context.Context) {
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			k.probeOnce(ctx)
		}
	}
}

// probeOnce 对所有 session 各发一次探测；pong 超时（interval）即摘除。
// 探测经由 MuxConn 的独立 reqID，与业务调用互不阻塞（多路复用）。
func (k *ProviderKeepalive) probeOnce(ctx context.Context) {
	for _, sess := range k.store.List() {
		conn := sess.Conn()
		if conn == nil || conn.IsClosed() {
			k.store.Remove(sess.SessionID)
			continue
		}
		req, err := proto.Marshal(&sdkv1.ProviderHeartbeatRequest{
			SessionId: sess.SessionID,
			ServiceId: sess.ServiceID,
		})
		if err != nil {
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, k.interval)
		_, _, perr := conn.Call(probeCtx, protocol.MsgProviderHeartbeatRequest, req)
		cancel()
		if perr != nil {
			// pong 超时/连接死：断开 + 摘除。SDK 侧重连逻辑会重新握手入池。
			k.logger.Warn("provider keepalive probe failed, removing session",
				"session_id", sess.SessionID, "service_id", sess.ServiceID, "error", perr)
			_ = sess.Close()
			k.store.Remove(sess.SessionID)
		}
	}
}
