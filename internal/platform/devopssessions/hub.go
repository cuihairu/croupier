// Package devopssessions devops 会话中继的 server 侧运营面（AGENT-002）：
// 面板经 server 代理向 devops agent 下行 open/input/resize/close（复用
// AgentSessionResolver 的既有会话解析），agent 上行的 DevopsSessionEvent
// 按 sessionID 扇出给面板消费者（SSE）。有界消费者队列 + 溢出摘除 +
// 回放环（背压语义见 relay wire 契约：面板跟不上时摘流不断会话）。
//
// 真值边界：授权与「谁」在 REST 层完成（operator 是 server 鉴权后的身份）；
// 白名单/审计前置/数据面背压在 agent 侧 Hub/Broker（不另立真值）；本包只
// 持有「server 在代理哪些会话」这一路由态。控制面丢失（agent 断连）=
// 该 agent 的全部代理会话收束（与 agent 侧 OnConnClosed 对称）。
package devopssessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// SessionResolver 会话解析（dispatch.AgentSessionResolver 的结构子集；
// ops 代理调用同源）。
type SessionResolver interface {
	ResolveAgentConn(agentID string) (transport.SessionCaller, bool)
}

// wireCallTimeout 单次下行会话请求预算（SSH 建链可能慢于 RPC 常规预算，
// 与控制面 RecvTimeout 30s 对齐留量）。
const wireCallTimeout = 15 * time.Second

// replayCap 回放环容量：SSE 重连接收最近 N 条事件（输出分片经 agent 侧
// CastChunk 截断，单条内存有界；256 条 ≈ 一屏回滚量级）。
const replayCap = 256

// consumerCap 单消费者队列深度。满载即摘除该消费者（会话与存档不受影响，
// 面板可重连拿回放）——路由循环绝不阻塞在慢消费者上。
const consumerCap = 256

// 消费者摘除原因（消费者通道关闭后可读，SSE 据此收尾）。
const (
	// EvictOverflow 消费者队列满载（面板跟不上输出速率）。
	EvictOverflow = "overflow"
	// EvictEnded 会话收束（主动关 / agent 上报 end / agent 断连兜底）。
	EvictEnded = "session_ended"
)

// ErrSessionBusy 输入背压哨兵：agent 侧输入队列满载超时（busyReason 的
// server 侧镜像，REST 映射 429）。
var ErrSessionBusy = fmt.Errorf("session busy")

// Session server 正在代理的一条 devops 会话的路由态（只读快照字段）。
type Session struct {
	SessionID string
	AgentID   string
	Operator  string
	Protocol  string
	Host      string
	Port      int32
	GameID    string
	Env       string
	Channel   string
	CreatedAt time.Time
}

// Subscription 面板消费者句柄（SSE 流）。C 关闭即流终止，读 EvictReason
// 区分收尾语义。
type Subscription struct {
	// C 事件通道（接收视图；关闭即流终止）。
	C <-chan *opsv1.DevopsSessionEvent
	// Replay 订阅时刻的回放快照（最近 replayCap 条，旧→新）。
	Replay []*opsv1.DevopsSessionEvent

	// ch 发送侧句柄（仅本包扇出/收束用）。
	ch         chan *opsv1.DevopsSessionEvent
	id         uint64
	session    *trackedSession
	evictMu    sync.Mutex
	evictState string
}

// EvictReason 返回摘除原因；流仍活着时为空串。
func (s *Subscription) EvictReason() string {
	s.evictMu.Lock()
	defer s.evictMu.Unlock()
	return s.evictState
}

func (s *Subscription) evict(reason string) {
	s.evictMu.Lock()
	s.evictState = reason
	s.evictMu.Unlock()
}

// trackedSession 一条代理会话的路由态 + 消费者集 + 回放环。
type trackedSession struct {
	meta Session

	mu        sync.Mutex
	consumers map[uint64]*Subscription
	replay    []*opsv1.DevopsSessionEvent
	nextSubID uint64
	ended     bool
}

// Hub server 侧会话中继运营面。nil 安全：REST 层对 nil hub 如实报
// 「未装配」。
type Hub struct {
	log *slog.Logger

	mu       sync.Mutex
	sessions map[string]*trackedSession // key: SessionID
	subSeq   uint64
}

// NewHub 装配中继运营面。
func NewHub(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{log: log, sessions: make(map[string]*trackedSession)}
}

// Open 经 agent 建立会话并登记路由态。会话 ID 由 server 生成（路由真值在
// server），wire 请求携带 operator（server 鉴权后的身份，agent 审计链落此
// 字段，不另信客户端自报）。
func (h *Hub) Open(ctx context.Context, resolver SessionResolver, agentID string, req *opsv1.DevopsSessionOpenRequest, operator, gameID, env string) (*Session, error) {
	if h == nil {
		return nil, fmt.Errorf("devops session relay not configured")
	}
	if resolver == nil {
		return nil, fmt.Errorf("session resolver unavailable")
	}
	caller, ok := resolver.ResolveAgentConn(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %q not found or no active connection", agentID)
	}
	sessionID, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	req.SessionId = sessionID
	resp := &opsv1.DevopsSessionOpenResponse{}
	if err := callWire(ctx, caller, protocol.MsgDevopsSessionOpenRequest, req, resp); err != nil {
		return nil, err
	}
	if !resp.GetOk() {
		return nil, fmt.Errorf("%s", resp.GetError())
	}
	ts := &trackedSession{
		meta: Session{
			SessionID: sessionID,
			AgentID:   agentID,
			Operator:  operator,
			Protocol:  req.GetProtocol(),
			Host:      req.GetHost(),
			Port:      req.GetPort(),
			GameID:    gameID,
			Env:       env,
			Channel:   resp.GetChannel(),
			CreatedAt: time.Now(),
		},
		consumers: make(map[uint64]*Subscription),
	}
	h.mu.Lock()
	h.sessions[sessionID] = ts
	h.mu.Unlock()
	h.log.Info("devops proxy session opened", "sessionId", sessionID, "agent", agentID,
		"operator", operator, "target", fmt.Sprintf("%s:%d", req.GetHost(), req.GetPort()))
	return &ts.meta, nil
}

// List 当前全部代理会话快照（旧→新按创建时间）。
func (h *Hub) List() []*Session {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	out := make([]*Session, 0, len(h.sessions))
	for _, ts := range h.sessions {
		meta := ts.meta
		out = append(out, &meta)
	}
	h.mu.Unlock()
	// 会话量小（面板并发开会话个位数）；排序保稳定展示。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.Before(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Get 单条代理会话快照。
func (h *Hub) Get(sessionID string) (*Session, bool) {
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ts, ok := h.sessions[sessionID]
	if !ok {
		return nil, false
	}
	meta := ts.meta
	return &meta, true
}

// AttachConsumer 订阅会话事件流（回放快照 + 实时通道）。会话不存在回 false。
func (h *Hub) AttachConsumer(sessionID string) (*Subscription, bool) {
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ts, ok := h.sessions[sessionID]
	if !ok {
		return nil, false
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.ended {
		return nil, false
	}
	id := ts.nextSubID
	ts.nextSubID++
	ch := make(chan *opsv1.DevopsSessionEvent, consumerCap)
	sub := &Subscription{
		C:       ch,
		Replay:  append([]*opsv1.DevopsSessionEvent(nil), ts.replay...),
		ch:      ch,
		id:      id,
		session: ts,
	}
	ts.consumers[id] = sub
	return sub, true
}

// DetachConsumer 主动退订（SSE 断开清理）。
func (h *Hub) DetachConsumer(sessionID string, sub *Subscription) {
	if h == nil || sub == nil {
		return
	}
	ts := sub.session
	if ts == nil {
		return
	}
	ts.mu.Lock()
	if cur, ok := ts.consumers[sub.id]; ok && cur == sub {
		delete(ts.consumers, sub.id)
	}
	ts.mu.Unlock()
}

// Input 下行键盘数据。背压（agent 侧输入队列满载）归一 ErrSessionBusy。
func (h *Hub) Input(ctx context.Context, resolver SessionResolver, sessionID string, data []byte) error {
	resp := &opsv1.DevopsSessionInputResponse{}
	err := h.callSession(ctx, resolver, sessionID, protocol.MsgDevopsSessionInputRequest,
		&opsv1.DevopsSessionInputRequest{SessionId: sessionID, Data: data}, resp)
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		if resp.GetError() == "busy" {
			return ErrSessionBusy
		}
		return fmt.Errorf("%s", resp.GetError())
	}
	return nil
}

// Resize 下行窗口调整。
func (h *Hub) Resize(ctx context.Context, resolver SessionResolver, sessionID string, cols, rows int32) error {
	resp := &opsv1.DevopsSessionResizeResponse{}
	return h.callSession(ctx, resolver, sessionID, protocol.MsgDevopsSessionResizeRequest,
		&opsv1.DevopsSessionResizeRequest{SessionId: sessionID, Cols: cols, Rows: rows}, resp)
}

// Close 主动断开：下行 close + 本地路由态收束（消费者收到 end 事件）。
func (h *Hub) Close(ctx context.Context, resolver SessionResolver, sessionID, reason string) error {
	resp := &opsv1.DevopsSessionCloseResponse{}
	err := h.callSession(ctx, resolver, sessionID, protocol.MsgDevopsSessionCloseRequest,
		&opsv1.DevopsSessionCloseRequest{SessionId: sessionID, Reason: reason}, resp)
	// wire 失败（含 agent 已不可达）仍收束本地态：会话真值在 agent 侧，
	// 控制不可达时代理会话没有续命意义（与控制面丢失=会话面关闭一致）。
	h.endSession(sessionID, "closed")
	return err
}

// RouteEvent agent 上行事件路由（ControlService 派发入口）：按 (agent,
// sessionID) 命中代理会话，追加回放环 + 非阻塞扇出；end 事件收束会话。
// 未知会话（清理竞态/重连旧帧）静默丢弃——事件面不是审计真值。
func (h *Hub) RouteEvent(agentID string, ev *opsv1.DevopsSessionEvent) {
	if h == nil || ev == nil {
		return
	}
	h.mu.Lock()
	ts, ok := h.sessions[ev.GetSessionId()]
	h.mu.Unlock()
	if !ok || ts.meta.AgentID != agentID {
		return
	}
	ts.mu.Lock()
	if ts.ended {
		ts.mu.Unlock()
		return
	}
	ts.replay = append(ts.replay, ev)
	if len(ts.replay) > replayCap {
		ts.replay = ts.replay[len(ts.replay)-replayCap:]
	}
	consumers := make([]*Subscription, 0, len(ts.consumers))
	for _, sub := range ts.consumers {
		consumers = append(consumers, sub)
	}
	isEnd := ev.GetKind() == "end"
	if isEnd {
		ts.ended = true
	}
	ts.mu.Unlock()

	for _, sub := range consumers {
		select {
		case sub.ch <- ev:
		default:
			// 慢消费者：摘除不断会话（背压语义——面板跟不上时丢的是面板
			// 流，不是会话数据；agent 侧存档仍是完整真值）。
			h.detachAndClose(ts, sub, EvictOverflow)
			h.log.Warn("devops session consumer evicted (overflow)",
				"sessionId", ev.GetSessionId(), "consumer", sub.id)
		}
	}
	if isEnd {
		h.closeAll(ts, EvictEnded)
		h.mu.Lock()
		delete(h.sessions, ev.GetSessionId())
		h.mu.Unlock()
	}
}

// CloseAgentSessions agent 断连兜底收束（TCPListener 断连清理入口）：该
// agent 的全部代理会话合成 end 事件扇出（与 agent 侧 OnConnClosed 对称，
// 控制面丢失 = 会话面关闭）。
func (h *Hub) CloseAgentSessions(agentID, reason string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	var mine []*trackedSession
	for id, ts := range h.sessions {
		if ts.meta.AgentID == agentID {
			mine = append(mine, ts)
			delete(h.sessions, id)
		}
	}
	h.mu.Unlock()
	for _, ts := range mine {
		h.endTracked(ts, "agent_disconnected", reason)
	}
	if len(mine) > 0 {
		h.log.Info("devops proxy sessions closed on agent disconnect",
			"agent", agentID, "count", len(mine))
	}
}

// endSession 会话收束（Close 入口）：合成 end 事件扇出 + 摘表。
func (h *Hub) endSession(sessionID, outcome string) {
	h.mu.Lock()
	ts, ok := h.sessions[sessionID]
	if ok {
		delete(h.sessions, sessionID)
	}
	h.mu.Unlock()
	if ok {
		h.endTracked(ts, outcome, "")
	}
}

// endTracked 合成 end 事件：先扇出给消费者（保序——end 前的输出已入队），
// 再收束全部消费者。
func (h *Hub) endTracked(ts *trackedSession, outcome, reason string) {
	ts.mu.Lock()
	if ts.ended {
		ts.mu.Unlock()
		return
	}
	ts.ended = true
	end := &opsv1.DevopsSessionEvent{
		SessionId: ts.meta.SessionID,
		Kind:      "end",
		Outcome:   outcome,
		Data:      []byte(reason),
	}
	ts.replay = append(ts.replay, end)
	if len(ts.replay) > replayCap {
		ts.replay = ts.replay[len(ts.replay)-replayCap:]
	}
	consumers := make([]*Subscription, 0, len(ts.consumers))
	for _, sub := range ts.consumers {
		consumers = append(consumers, sub)
	}
	ts.mu.Unlock()

	for _, sub := range consumers {
		select {
		case sub.ch <- end:
		default:
		}
	}
	h.closeAll(ts, EvictEnded)
}

// closeAll 收束会话的全部消费者（通道关闭，EvictReason=session_ended）。
func (h *Hub) closeAll(ts *trackedSession, reason string) {
	ts.mu.Lock()
	consumers := make([]*Subscription, 0, len(ts.consumers))
	for _, sub := range ts.consumers {
		consumers = append(consumers, sub)
	}
	ts.consumers = make(map[uint64]*Subscription)
	ts.mu.Unlock()
	for _, sub := range consumers {
		sub.evict(reason)
		close(sub.ch)
	}
}

// detachAndClose 摘除单个消费者并关闭其通道。
func (h *Hub) detachAndClose(ts *trackedSession, sub *Subscription, reason string) {
	ts.mu.Lock()
	if cur, ok := ts.consumers[sub.id]; ok && cur == sub {
		delete(ts.consumers, sub.id)
	}
	ts.mu.Unlock()
	sub.evict(reason)
	close(sub.ch)
}

// callSession 命中会话属主的下行请求（input/resize/close）：agentID 从
// 路由态取，REST 层已按会话属主做过 scope 校验。
func (h *Hub) callSession(ctx context.Context, resolver SessionResolver, sessionID string, msgID uint32, req, resp proto.Message) error {
	if h == nil {
		return fmt.Errorf("devops session relay not configured")
	}
	if resolver == nil {
		return fmt.Errorf("session resolver unavailable")
	}
	h.mu.Lock()
	ts, ok := h.sessions[sessionID]
	h.mu.Unlock()
	if !ok {
		return ErrSessionNotFound
	}
	caller, ok := resolver.ResolveAgentConn(ts.meta.AgentID)
	if !ok {
		return fmt.Errorf("agent %q not found or no active connection", ts.meta.AgentID)
	}
	return callWire(ctx, caller, msgID, req, resp)
}

// ErrSessionNotFound 未知会话（REST 映射 404）。
var ErrSessionNotFound = fmt.Errorf("session not found")

// callWire 单次下行 wire 调用（超时预算内等响应帧；解帧失败即协议错）。
func callWire(ctx context.Context, caller transport.SessionCaller, msgID uint32, req, resp proto.Message) error {
	ctx, cancel := context.WithTimeout(ctx, wireCallTimeout)
	defer cancel()
	body, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", protocol.MsgIDString(msgID), err)
	}
	_, respBody, err := caller.Call(ctx, msgID, body)
	if err != nil {
		return fmt.Errorf("%s: %w", protocol.MsgIDString(msgID), err)
	}
	if err := proto.Unmarshal(respBody, resp); err != nil {
		return fmt.Errorf("unmarshal %s response: %w", protocol.MsgIDString(msgID), err)
	}
	return nil
}

// newSessionID 服务端生成的会话 ID（dops- + 16 hex 随机）。
func newSessionID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dops-" + hex.EncodeToString(buf), nil
}
