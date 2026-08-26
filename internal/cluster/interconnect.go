package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cuihairu/croupier/pkg/protocol"
)

// MeshInterconnect 是直连 mesh 实现：懒建连 + 连接池 + 一跳限制 + fencing。
//
// 传输复用 transport/tcp 的帧协议；消息体为 JSON（互联是内部低频路径，
// JSON 足够；帧协议与既有链路完全一致，mTLS 由传输层配置承接）。
type MeshInterconnect struct {
	self       PeerInfo
	selfEpoch  uint64
	resolver   OwnerResolver
	membership Membership

	mu    sync.Mutex
	conns map[string]*peerConn // key: instanceID

	dial  dialFunc
	hello helloFunc

	// peersCache：last-known peers（共享存储抖动时继续转发）。
	peersMu    sync.RWMutex
	peersCache []PeerInfo
}

// dialFunc 建立/复用到对端的连接并完成 server 握手（可注入测试替身）。
type dialFunc func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error)

// helloFunc 发送握手（dial 内部使用；拆出来便于测试）。
type helloFunc func(conn *peerConn, self PeerInfo) error

// peerConn 是一条到对端的长连接包装。
type peerConn struct {
	instanceID string
	addr       string
	epoch      uint64

	send  func(ctx context.Context, msgID uint32, body []byte) ([]byte, error)
	close func()
}

// NewMeshInterconnect 创建 mesh 互联。
func NewMeshInterconnect(self PeerInfo, resolver OwnerResolver, membership Membership) *MeshInterconnect {
	m := &MeshInterconnect{
		self:       self,
		resolver:   resolver,
		membership: membership,
		conns:      map[string]*peerConn{},
	}
	m.dial = defaultDialPeer
	return m
}

// SetEpoch 设置 Register 返回的本实例 epoch（fencing 校验用）。
func (m *MeshInterconnect) SetEpoch(epoch uint64) { m.selfEpoch = epoch }

// SelfInfo 返回本实例身份。
func (m *MeshInterconnect) SelfInfo() PeerInfo { return m.self }

// RefreshPeers 刷新 last-known peers 缓存（发现循环调用）。
func (m *MeshInterconnect) RefreshPeers(ctx context.Context) {
	if m.membership == nil {
		return
	}
	peers, err := m.membership.ListAlive(ctx)
	if err != nil {
		slog.Warn("cluster: refresh peers failed, keeping last-known", "error", err)
		return
	}
	m.peersMu.Lock()
	m.peersCache = peers
	m.peersMu.Unlock()
}

// Peers 返回 last-known peers（排除自身）。
func (m *MeshInterconnect) Peers() []PeerInfo {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	out := make([]PeerInfo, 0, len(m.peersCache))
	for _, p := range m.peersCache {
		if p.InstanceID != m.self.InstanceID {
			out = append(out, p)
		}
	}
	return out
}

// Forward 转发调用到 owner 实例。
func (m *MeshInterconnect) Forward(ctx context.Context, agentID string, req *ForwardedInvoke) (*ForwardedResult, error) {
	// 铁律一：一跳限制——已被转发过的请求不再转发。
	if req.Forwarded {
		return nil, ErrHopLimit
	}
	if m.resolver == nil {
		return nil, ErrNoRoute
	}
	owner, err := m.resolver.ResolveOwner(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("resolve owner: %w", err)
	}
	if owner == nil {
		return nil, ErrNoRoute
	}
	// 目标是自己：调用方不应进入转发路径，防御性返回。
	if owner.InstanceID == m.self.InstanceID {
		return &ForwardedResult{OK: false, Error: "owner is self"}, nil
	}

	req.Forwarded = true
	req.AgentID = agentID
	// 记录解析时的 owner epoch（owner 侧 fencing 校验）。
	req.CallerEpoch = owner.Epoch

	conn, err := m.getConn(ctx, *owner)
	if err != nil {
		return nil, fmt.Errorf("dial owner %s(%s): %w", owner.InstanceID, owner.AdvertiseAddr, err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	respBody, err := conn.send(ctx, protocol.MsgForwardInvokeReq, body)
	if err != nil {
		m.dropConn(owner.InstanceID)
		return nil, fmt.Errorf("forward rpc: %w", err)
	}
	var result ForwardedResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode forward response: %w", err)
	}
	return &result, nil
}

// getConn 懒建连/复用到 owner 的连接。
func (m *MeshInterconnect) getConn(ctx context.Context, owner PeerInfo) (*peerConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[owner.InstanceID]; ok && c.addr == owner.AdvertiseAddr {
		return c, nil
	}
	if c, ok := m.conns[owner.InstanceID]; ok {
		c.close()
		delete(m.conns, owner.InstanceID)
	}
	c, err := m.dial(ctx, owner.AdvertiseAddr, m.self)
	if err != nil {
		return nil, err
	}
	c.instanceID = owner.InstanceID
	c.epoch = owner.Epoch
	m.conns[owner.InstanceID] = c
	return c, nil
}

func (m *MeshInterconnect) dropConn(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[instanceID]; ok {
		c.close()
		delete(m.conns, instanceID)
	}
}

// Close 关闭全部对端连接。
func (m *MeshInterconnect) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.conns {
		c.close()
		delete(m.conns, id)
	}
}

// defaultDialPeer 通过 transport/tcp 基座建立到对端的连接并握手。
func defaultDialPeer(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
	return nil, fmt.Errorf("cluster: tcp dial to %s not wired in test context", addr)
}

var _ Interconnect = (*MeshInterconnect)(nil)

// ServeForwardHandler 返回 owner 侧处理 MsgForwardInvokeReq 的处理函数。
// localInvoke 由 dispatch 层注入：校验 caller 鉴权后走本地 Agent 连接执行。
//
// fencing 铁律二在此执行：req.CallerEpoch ≠ 本地 epoch 时拒绝
// （网络分区恢复后的僵尸 owner 场景）。
func ServeForwardHandler(selfEpoch uint64, localInvoke func(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error)) func(ctx context.Context, body []byte) []byte {
	return func(ctx context.Context, body []byte) []byte {
		var req ForwardedInvoke
		if err := json.Unmarshal(body, &req); err != nil {
			return marshalResult(&ForwardedResult{OK: false, Error: "bad request"})
		}
		// 铁律一：到达 owner 的已转发请求不得再次转发（由调用方保证 Forwarded=true，
		// owner 侧仅拒绝显式环路标记外的异常）。
		if !req.Forwarded {
			return marshalResult(&ForwardedResult{OK: false, Error: "not forwarded flag"})
		}
		// 铁律二：fencing——发起方看到的 epoch 与本地不一致 → 僵尸 owner。
		if req.CallerEpoch != selfEpoch {
			return marshalResult(&ForwardedResult{OK: false, NotOwner: true, Error: "stale epoch"})
		}
		result, err := localInvoke(ctx, &req)
		if err != nil {
			return marshalResult(&ForwardedResult{OK: false, Error: err.Error()})
		}
		return marshalResult(result)
	}
}

func marshalResult(r *ForwardedResult) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		return []byte(`{"ok":false,"error":"marshal"}`)
	}
	return b
}

// Now is exposed for tests.
var Now = time.Now
