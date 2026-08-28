// Package cluster 实现 Server 多实例高可用的实例互联
// （docs/architecture/server-ha-multi-instance.md）。
//
// 组件：
//   - Membership：instances 成员表（自注册 + 租约 + 轮询发现），
//     存储复用共享目录（RegistryStore 的 DB/Redis），无 seed、无静态 peers
//   - Interconnect：转发接口 + 直连 mesh 实现（懒建连 + 一跳限制 + fencing）
//
// 传输复用 internal/transport/tcp 基座（帧协议 + 消息 ID 路由），
// 握手角色 server（MsgServerHelloRequest），转发消息 MsgForwardInvokeReq。
package cluster

import (
	"context"
	"errors"
	"time"
)

// ErrNotOwner 表示转发目标实例已不是该 Agent 的 owner（目录过期）。
// 调用方应重新解析目录后重试一次。
var ErrNotOwner = errors.New("cluster: peer is not the owner")

// ErrStaleEpoch 表示目标实例是僵尸 owner（fencing 校验失败）。
var ErrStaleEpoch = errors.New("cluster: stale owner epoch")

// ErrNoRoute 表示目录中找不到持有该 Agent 的存活实例（如 Agent 已离线）。
var ErrNoRoute = errors.New("cluster: no live owner for agent")

// ErrHopLimit 表示请求已被转发过一次，拒绝二次转发（防环路）。
var ErrHopLimit = errors.New("cluster: forward hop limit exceeded")

// PeerInfo 是一个存活对端的描述。
type PeerInfo struct {
	InstanceID    string    `json:"instanceId"`
	AdvertiseAddr string    `json:"advertiseAddr"`
	Epoch         uint64    `json:"epoch"`
	StartedAt     time.Time `json:"startedAt"`
}

// CallerContext 携带原始调用者身份，owner 侧必须据此重新鉴权。
type CallerContext struct {
	AdminID  uint     `json:"adminId"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	GameID   string   `json:"gameId"`
	Env      string   `json:"env"`
	TraceID  string   `json:"traceId"`
}

// ForwardedInvoke 是转发到 owner 实例的调用请求。
type ForwardedInvoke struct {
	AgentID    string `json:"agentId"`
	FunctionID string `json:"functionId"`
	Payload    []byte `json:"payload"`
	// Metadata 与 InvokeRequest.Metadata 同构（route/service_id 等）。
	Metadata map[string]string `json:"metadata,omitempty"`
	// IdempotencyKey 可选幂等键，透传执行路径。
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	// Forwarded 必须为 false；owner 收到 true 时拒绝（一跳限制）。
	Forwarded bool `json:"forwarded"`
	// CallerEpoch 发起转发方解析目录时看到的 owner epoch，
	// owner 侧对比本地 epoch 实现 fencing（不一致 = 僵尸 owner）。
	CallerEpoch uint64        `json:"callerEpoch"`
	Caller      CallerContext `json:"caller"`
	TimeoutMs   int64         `json:"timeoutMs,omitempty"`
}

// ForwardedResult 是转发调用的结果。
type ForwardedResult struct {
	OK bool `json:"ok"`
	// Payload 是 proto 编码的 InvokeResponse 字节。曾用 json.RawMessage
	// 承载——proto 字节不是合法 JSON，owner 侧 json.Marshal 必失败并
	// 返回无意义的 "marshal" 错误（线上跨实例调用全挂的根因）。
	Payload []byte `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
	// NotOwner 为 true 时调用方应重解析目录重试。
	NotOwner bool `json:"notOwner,omitempty"`
}

// Interconnect 是实例间转发服务抽象（dispatch 层唯一依赖）。
type Interconnect interface {
	// Forward 把调用请求转给持有目标 Agent 连接的实例。
	// 实现内部：解析 owner → 懒建连/复用 → 发送 → 一跳/fencing 校验。
	Forward(ctx context.Context, agentID string, req *ForwardedInvoke) (*ForwardedResult, error)
	// Peers 返回当前已知存活对端。
	Peers() []PeerInfo
}

// OwnerResolver 从共享目录解析 agent → owner 实例。
// 由 RegistryStore 提供实现（memory 单实例直返自身，db/redis 走共享表）。
type OwnerResolver interface {
	// ResolveOwner 返回持有该 Agent 连接的实例；无存活 owner 返回 (nil, nil)。
	ResolveOwner(ctx context.Context, agentID string) (*PeerInfo, error)
}

// Membership 是成员表操作（自注册/心跳/发现）。
type Membership interface {
	// Register 自注册并返回分配的 epoch。
	Register(ctx context.Context, info PeerInfo) (uint64, error)
	// Renew 续租。
	Renew(ctx context.Context, instanceID string) error
	// ListAlive 返回租约未过期的全部成员（含自身）。
	ListAlive(ctx context.Context) ([]PeerInfo, error)
	// Resign 优雅退出（清掉自己的记录）。
	Resign(ctx context.Context, instanceID string) error
}
