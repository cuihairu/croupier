// Package cluster implements multi-instance HA interconnect for the Server
// (docs/architecture/server-ha-multi-instance.md).
//
// Components:
//   - Membership: the instances member table (self registration + lease +
//     polling discovery); storage reuses the shared directory (RegistryStore's
//     DB/Redis), with no seed nodes and no static peers
//   - Interconnect: a forwarding interface plus a direct-connection mesh
//     implementation (lazy connection setup + one-hop limit + fencing)
//
// Transport reuses the internal/transport/tcp base (frame protocol + message
// ID routing); the handshake role is server (MsgServerHelloRequest) and the
// forward message is MsgForwardInvokeReq.
package cluster

import (
	"context"
	"errors"
	"time"
)

// ErrNoRoute means no live instance holding that Agent was found in the
// directory (e.g. the Agent is offline).
var ErrNoRoute = errors.New("cluster: no live owner for agent")

// ErrHopLimit means the request has already been forwarded once; a second hop
// is refused (loop prevention).
var ErrHopLimit = errors.New("cluster: forward hop limit exceeded")

// PeerInfo describes one live peer.
type PeerInfo struct {
	InstanceID    string    `json:"instanceId"`
	AdvertiseAddr string    `json:"advertiseAddr"`
	Epoch         uint64    `json:"epoch"`
	StartedAt     time.Time `json:"startedAt"`
}

// CallerContext carries the original caller identity; the owner side must
// re-authenticate based on it.
type CallerContext struct {
	AdminID  uint     `json:"adminId"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	GameID   string   `json:"gameId"`
	Env      string   `json:"env"`
	TraceID  string   `json:"traceId"`
}

// Forward kinds: one frame format carries all three call kinds (loop
// prevention/fencing/stamping share it); an empty Kind is treated as invoke
// (older callers leave it unset and the owner treats it as invoke).
const (
	ForwardKindInvoke    = ""
	ForwardKindStartTask = "start_task"
	ForwardKindCancel    = "cancel_task"
)

// ForwardedInvoke is a call request forwarded to the owner instance.
type ForwardedInvoke struct {
	AgentID    string `json:"agentId"`
	FunctionID string `json:"functionId"`
	Payload    []byte `json:"payload"`
	// Metadata is shaped like InvokeRequest.Metadata (route/service_id etc.).
	Metadata map[string]string `json:"metadata,omitempty"`
	// IdempotencyKey is an optional idempotency key, passed through to the
	// execution path.
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	// Kind distinguishes forwarded call kinds (invoke/start_task/cancel_task);
	// empty = invoke (synchronous invocation was the first consumer of the
	// forward path, kept for backward compatibility).
	Kind string `json:"kind,omitempty"`
	// TaskID is the target task ID for cancel_task. For start_task the task ID
	// travels in Metadata.taskId, passed through with InvokeRequest semantics
	// (the agent side parses it the same way).
	TaskID string `json:"taskId,omitempty"`
	// Forwarded must be false; the owner rejects true (one-hop limit).
	Forwarded bool `json:"forwarded"`
	// CallerEpoch is the owner epoch the forwarding side saw when resolving
	// the directory; the owner compares it with the local epoch for fencing
	// (mismatch = zombie owner).
	CallerEpoch uint64        `json:"callerEpoch"`
	Caller      CallerContext `json:"caller"`
	TimeoutMs   int64         `json:"timeoutMs,omitempty"`
}

// ForwardedResult is the result of a forwarded call.
type ForwardedResult struct {
	OK bool `json:"ok"`
	// Payload holds the proto-encoded InvokeResponse bytes. It used to be
	// carried as json.RawMessage — proto bytes are not valid JSON, so the
	// owner's json.Marshal always failed and returned a meaningless "marshal"
	// error (the root cause of cross-instance calls failing wholesale in
	// production).
	Payload []byte `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
	// NotOwner, when true, tells the caller to re-resolve the directory and
	// retry.
	NotOwner bool `json:"notOwner,omitempty"`
}

// Interconnect is the inter-instance forwarding service abstraction (the only
// dependency the dispatch layer takes).
type Interconnect interface {
	// Forward hands the call request to the instance that holds the target
	// Agent's connection.
	// Internally: resolve owner → lazy connect/reuse → send → one-hop/fencing
	// checks.
	Forward(ctx context.Context, agentID string, req *ForwardedInvoke) (*ForwardedResult, error)
	// Peers returns the currently known live peers.
	Peers() []PeerInfo
}

// OwnerResolver resolves agent → owner instance from the shared directory.
// RegistryStore provides implementations (memory returns self directly on a
// single instance; db/redis go through the shared table).
type OwnerResolver interface {
	// ResolveOwner returns the instance holding that Agent's connection;
	// with no live owner it returns (nil, nil).
	ResolveOwner(ctx context.Context, agentID string) (*PeerInfo, error)
}

// Membership is the member-table API (self registration/heartbeat/discovery).
type Membership interface {
	// Register self-registers and returns the assigned epoch.
	Register(ctx context.Context, info PeerInfo) (uint64, error)
	// Renew renews the lease.
	Renew(ctx context.Context, instanceID string) error
	// ListAlive returns all members whose leases have not expired (self
	// included).
	ListAlive(ctx context.Context) ([]PeerInfo, error)
	// Resign leaves gracefully (removing this instance's own record).
	Resign(ctx context.Context, instanceID string) error
}
