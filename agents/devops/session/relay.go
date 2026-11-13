package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// AGENT-002 会话中继面：把 server 下行的会话控制请求（open/input/resize/
// close）接到本包 Broker/Hub 建立链与数据面，把会话输出/结束以单发事件帧
// 上行。授权与「谁」在 server 侧完成（operator 是 server 鉴权后的身份）；
// 白名单/审计前置/背压/断线清理全部沿用 Hub 既有链路，中继不另立真值。
//
// 控制面丢失（连接断）= 会话面关闭（OnConnClosed → Hub.Close）：堡垒机
// 安全缺省——operator 不再有可审计授权通道时，交互会话不续命。重连后
// server 需重新开 会话；本包不跨连接续会话。

// Relay server 下行会话请求的 agent 侧处理器（transportcore.Handler）。
type Relay struct {
	hub *Hub
	log *slog.Logger

	// send 上行事件通道（runner 随重连换新；nil = 控制面未连，输出写即
	// 报错——消费者被 Attach 超时摘除，会话与存档不受影响）。
	sendMu sync.RWMutex
	send   func(ctx context.Context, msgID uint32, msg proto.Message) error

	// hubMu 保护 hub 的装配期换绑（注册先行、hub 后建）。
	hubMu sync.RWMutex
}

// NewRelay 装配中继并挂会话收束回调（end 事件上行）。hub 可为 nil（runner
// 注册期装配：会话面未启用时 open 如实回「会话面关闭」），随后 SetHub 接通。
func NewRelay(hub *Hub, log *slog.Logger) *Relay {
	if log == nil {
		log = slog.Default()
	}
	r := &Relay{hub: hub, log: log}
	if hub != nil {
		r.bindHub(hub)
	}
	return r
}

// SetHub 接通会话面（runner 在 Hub 装配后调用；nil = 摘除，open 拒绝）。
func (r *Relay) SetHub(hub *Hub) {
	r.hubMu.Lock()
	r.hub = hub
	r.hubMu.Unlock()
	if hub != nil {
		r.bindHub(hub)
	}
}

// hubSnapshot 当前会话面（Handle 车道 worker 并发读、SetHub 装配线程写）。
func (r *Relay) hubSnapshot() *Hub {
	r.hubMu.RLock()
	defer r.hubMu.RUnlock()
	return r.hub
}

// bindHub 挂会话收束回调（end 事件上行）。仅装配线程调用（Run 装配段）。
func (r *Relay) bindHub(hub *Hub) {
	hub.SetEndCallback(func(sessionID, outcome string) {
		r.sendEvent(&opsv1.DevopsSessionEvent{SessionId: sessionID, Kind: "end", Outcome: outcome})
	})
}

// SetSender 换绑上行通道（重连后由 runner 调；nil = 断连）。
func (r *Relay) SetSender(send func(ctx context.Context, msgID uint32, msg proto.Message) error) {
	r.sendMu.Lock()
	r.send = send
	r.sendMu.Unlock()
}

// sender 快照读取。
func (r *Relay) sender() func(ctx context.Context, msgID uint32, msg proto.Message) error {
	r.sendMu.RLock()
	defer r.sendMu.RUnlock()
	return r.send
}

// sendEvent 上行一条会话事件（尽力而为：控制面未连或写失败即丢——输出
// 面不是审计真值，审计走 execlog；end 事件同样尽力而为，server 侧还有
// 断连兜底收束）。
func (r *Relay) sendEvent(ev *opsv1.DevopsSessionEvent) {
	send := r.sender()
	if send == nil {
		return
	}
	if err := send(context.Background(), protocol.MsgDevopsSessionEvent, ev); err != nil {
		r.log.Debug("devops relay event send failed", "sessionId", ev.GetSessionId(),
			"kind", ev.GetKind(), "error", err.Error())
	}
}

// Handle 实现 transportcore.Handler：server 下行会话请求 → 建立链/数据面，
// 回帧响应。未知消息回空帧（不回错误——回错误会让读循环按协议违规断链），
// server 侧 Call 以解帧失败报「不支持」。
func (r *Relay) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgDevopsSessionOpenRequest:
		req := &opsv1.DevopsSessionOpenRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return proto.Marshal(&opsv1.DevopsSessionOpenResponse{Ok: false, Error: "bad request"})
		}
		return proto.Marshal(r.open(ctx, req))
	case protocol.MsgDevopsSessionInputRequest:
		req := &opsv1.DevopsSessionInputRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return proto.Marshal(&opsv1.DevopsSessionInputResponse{Ok: false, Error: "bad request"})
		}
		return proto.Marshal(r.input(req))
	case protocol.MsgDevopsSessionResizeRequest:
		req := &opsv1.DevopsSessionResizeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return proto.Marshal(&opsv1.DevopsSessionResizeResponse{Ok: false, Error: "bad request"})
		}
		return proto.Marshal(r.resize(req))
	case protocol.MsgDevopsSessionCloseRequest:
		req := &opsv1.DevopsSessionCloseRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return proto.Marshal(&opsv1.DevopsSessionCloseResponse{Ok: false, Error: "bad request"})
		}
		return proto.Marshal(r.close(req))
	default:
		return nil, nil
	}
}

// open 走完整建立链（校验→白名单→审计前置→通道 Open→存档+数据泵），并把
// 输出消费者挂到上行事件流。
func (r *Relay) open(ctx context.Context, req *opsv1.DevopsSessionOpenRequest) *opsv1.DevopsSessionOpenResponse {
	hub := r.hubSnapshot()
	if hub == nil {
		return &opsv1.DevopsSessionOpenResponse{Ok: false, Error: "devops session plane disabled"}
	}
	sess, err := hub.Open(ctx, SessionRequest{
		SessionID: req.GetSessionId(),
		Protocol:  Protocol(req.GetProtocol()),
		Host:      req.GetHost(),
		Port:      int(req.GetPort()),
		Operator:  req.GetOperator(),
		Cols:      int(req.GetCols()),
		Rows:      int(req.GetRows()),
	})
	if err != nil {
		return &opsv1.DevopsSessionOpenResponse{Ok: false, Error: err.Error()}
	}
	// 输出扇出 → 上行事件帧（chunk 已被 CastChunk 截断，单帧内存有界）。
	detach, ok := sess.Attach(&relayOutputWriter{relay: r, sessionID: sess.ID()})
	if !ok {
		_ = sess.Close()
		return &opsv1.DevopsSessionOpenResponse{Ok: false, Error: "session closed while attaching"}
	}
	_ = detach // 会话收束时 cleanup 统一摘除全部消费者，无需显式 detach
	return &opsv1.DevopsSessionOpenResponse{
		Ok:        true,
		SessionId: sess.ID(),
		Channel:   sess.Meta().Channel,
	}
}

// input 键盘数据入有界队列（满载超时回 busy——背压原样透传给 server/面板）。
func (r *Relay) input(req *opsv1.DevopsSessionInputRequest) *opsv1.DevopsSessionInputResponse {
	ls, ok := r.hubSnapshot().Get(req.GetSessionId())
	if !ok {
		return &opsv1.DevopsSessionInputResponse{Ok: false, Error: "session not found"}
	}
	if err := ls.Write(req.GetData()); err != nil {
		return &opsv1.DevopsSessionInputResponse{Ok: false, Error: busyReason(err)}
	}
	return &opsv1.DevopsSessionInputResponse{Ok: true}
}

// resize 调整远端 pty 窗口并存档事件（回放跟随）。
func (r *Relay) resize(req *opsv1.DevopsSessionResizeRequest) *opsv1.DevopsSessionResizeResponse {
	ls, ok := r.hubSnapshot().Get(req.GetSessionId())
	if !ok {
		return &opsv1.DevopsSessionResizeResponse{Ok: false, Error: "session not found"}
	}
	if err := ls.Resize(int(req.GetCols()), int(req.GetRows())); err != nil {
		return &opsv1.DevopsSessionResizeResponse{Ok: false, Error: err.Error()}
	}
	return &opsv1.DevopsSessionResizeResponse{Ok: true}
}

// close 主动断开并触发完整断线清理链（end 事件经收束回调上行）。
func (r *Relay) close(req *opsv1.DevopsSessionCloseRequest) *opsv1.DevopsSessionCloseResponse {
	ls, ok := r.hubSnapshot().Get(req.GetSessionId())
	if !ok {
		return &opsv1.DevopsSessionCloseResponse{Ok: false, Error: "session not found"}
	}
	if err := ls.Close(); err != nil {
		return &opsv1.DevopsSessionCloseResponse{Ok: false, Error: err.Error()}
	}
	return &opsv1.DevopsSessionCloseResponse{Ok: true}
}

// OnConnClosed 控制面丢失收束：关闭全部活跃会话（清理链含结束审计与 end
// 事件上行尽力而为）。server 重连后须重新开会话。
func (r *Relay) OnConnClosed() {
	hub := r.hubSnapshot()
	if hub == nil {
		return
	}
	if err := hub.Close(); err != nil {
		r.log.Warn("devops relay closed sessions on control loss", "error", err.Error())
	}
}

// busyReason 归一输入背压语义：满载超时（ErrSessionBusy 哨兵）报 "busy"，
// 其余原样（session closed 等）。
func busyReason(err error) string {
	if errors.Is(err, ErrSessionBusy) {
		return "busy"
	}
	return err.Error()
}

// relayOutputWriter 会话输出 → 上行事件帧的消费者写出口（Attach 挂载）。
// 控制面未连时 Write 报错 → 消费者被摘除，会话与存档不受影响。
type relayOutputWriter struct {
	relay     *Relay
	sessionID string
}

func (w *relayOutputWriter) Write(p []byte) (int, error) {
	send := w.relay.sender()
	if send == nil {
		return 0, fmt.Errorf("control conn not connected")
	}
	ev := &opsv1.DevopsSessionEvent{SessionId: w.sessionID, Kind: "output", Data: append([]byte(nil), p...)}
	if err := send(context.Background(), protocol.MsgDevopsSessionEvent, ev); err != nil {
		return 0, err
	}
	return len(p), nil
}
