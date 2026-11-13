// devops 会话中继面板面（AGENT-002）：面板经 server 代理向 devops agent
// 下行开/输入/调整/关 SSH 会话，输出经 SSE 流回面板。授权与 scope 在本层
// 完成（operator 取 server 鉴权身份，随 open 下发给 agent 审计链）；白名单/
// 数据面审计/背压真值在 agent 侧（Broker/Hub），本层只做路由与留痕——
// open/close 落 server 审计（devops.session_open/close），键入输出不落
// server 审计（agent execlog 是完整真值，双写只会产生两份漂移账）。
package ops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/devopssessions"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
)

// errDevopsRelayNotConfigured hub 未装配（devops 中继未接线）时的 503。
var errDevopsRelayNotConfigured = &errorx.CodeError{
	Code:       http.StatusServiceUnavailable,
	Message:    "devops session relay not configured",
	StableCode: "devops_relay_not_configured",
}

// ---- DTO（契约键 lowerCamelCase）----

// DevopsSessionOpenRequest 经指定 devops agent 开一条到 host:port 的会话。
type DevopsSessionOpenRequest struct {
	AgentID  string `json:"agentId"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int32  `json:"port"`
	Cols     int32  `json:"cols,omitempty"`
	Rows     int32  `json:"rows,omitempty"`
}

// DevopsSessionOpenResponse 开会话结果（会话 ID 由 server 生成）。
type DevopsSessionOpenResponse struct {
	SessionID string `json:"sessionId"`
	AgentID   string `json:"agentId"`
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int32  `json:"port"`
	Channel   string `json:"channel,omitempty"`
}

// DevopsSessionItem 代理会话列表条目。
type DevopsSessionItem struct {
	SessionID string `json:"sessionId"`
	AgentID   string `json:"agentId"`
	Operator  string `json:"operator"`
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int32  `json:"port"`
	Channel   string `json:"channel,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// DevopsSessionListResponse 代理会话列表。
type DevopsSessionListResponse struct {
	Sessions []DevopsSessionItem `json:"sessions"`
}

// DevopsSessionInputRequest 键盘数据（文本；回车含 \n）。
type DevopsSessionInputRequest struct {
	Data string `json:"data"`
}

// DevopsSessionResizeRequest 终端窗口（列 × 行）。
type DevopsSessionResizeRequest struct {
	Cols int32 `json:"cols"`
	Rows int32 `json:"rows"`
}

// DevopsSessionCloseRequest 主动关闭原因（审计留痕；无 body DELETE 也合法）。
type DevopsSessionCloseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ---- handlers ----

// DevopsSessionOpen handles POST /ops/devops/sessions.
func (h *Handler) DevopsSessionOpen(c *gin.Context) {
	var req DevopsSessionOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DevopsSessionOpen(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsSessionsList handles GET /ops/devops/sessions.
func (h *Handler) DevopsSessionsList(c *gin.Context) {
	resp, err := h.service.DevopsSessionsList(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsSessionStream handles GET /ops/devops/sessions/:sessionId/stream.
// SSE 豁免 JSON envelope 契约：event=事件种类（output/end），data=JSON
// （输出 base64：pty 字节流不保证 UTF-8）。
func (h *Handler) DevopsSessionStream(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := h.service.DevopsSessionStreamAuthorize(c.Request.Context(), sessionID); err != nil {
		response.Error(c, err)
		return
	}
	sub, ok := h.service.AttachDevopsSessionStream(sessionID)
	if !ok {
		response.Error(c, errorx.NewNotFound("devops session not found: "+sessionID))
		return
	}
	defer h.service.DetachDevopsSessionStream(sessionID, sub)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(http.StatusOK)

	// 回放快照先行（SSE 重连不丢最近一屏）。
	for _, ev := range sub.Replay {
		writeDevopsSessionEvent(c, ev)
	}
	// Use the request context — gin cancels it when the client disconnects.
	ctx := c.Request.Context()
	for {
		select {
		case ev, ok := <-sub.C:
			if !ok {
				// 摘除（溢出/会话收束）：补 end 语义收尾帧，面板据此断开。
				if sub.EvictReason() == devopssessions.EvictOverflow {
					writeDevopsSessionEnd(c, sessionID, "overflow", "consumer too slow")
				}
				return
			}
			writeDevopsSessionEvent(c, ev)
			if ev.GetKind() == "end" {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// DevopsSessionInput handles POST /ops/devops/sessions/:sessionId/input.
// 背压映射 429 session_busy（瞬态流控而非状态冲突，不用 409 的语义面）。
func (h *Handler) DevopsSessionInput(c *gin.Context) {
	var req DevopsSessionInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.DevopsSessionInput(c.Request.Context(), c.Param("sessionId"), req.Data); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DevopsSessionResize handles POST /ops/devops/sessions/:sessionId/resize.
func (h *Handler) DevopsSessionResize(c *gin.Context) {
	var req DevopsSessionResizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.DevopsSessionResize(c.Request.Context(), c.Param("sessionId"), req.Cols, req.Rows); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DevopsSessionClose handles DELETE /ops/devops/sessions/:sessionId.
func (h *Handler) DevopsSessionClose(c *gin.Context) {
	var req DevopsSessionCloseRequest
	_ = c.ShouldBindJSON(&req) // body 可选
	if err := h.service.DevopsSessionClose(c.Request.Context(), c.Param("sessionId"), req.Reason); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeDevopsSessionEvent 单条 SSE 事件帧（marshal 失败丢帧不中断流）。
func writeDevopsSessionEvent(c *gin.Context, ev *opsv1.DevopsSessionEvent) {
	payload, err := json.Marshal(devopsSSEPayload{
		SessionID: ev.GetSessionId(),
		Kind:      ev.GetKind(),
		Data:      base64.StdEncoding.EncodeToString(ev.GetData()),
		Outcome:   ev.GetOutcome(),
	})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.GetKind(), payload)
	c.Writer.Flush()
}

// writeDevopsSessionEnd 消费者被摘除（无 end 帧可放行）时补发的收尾帧。
func writeDevopsSessionEnd(c *gin.Context, sessionID, outcome, reason string) {
	payload, _ := json.Marshal(devopsSSEPayload{
		SessionID: sessionID,
		Kind:      "end",
		Outcome:   outcome,
		Data:      base64.StdEncoding.EncodeToString([]byte(reason)),
	})
	_, _ = fmt.Fprintf(c.Writer, "event: end\ndata: %s\n\n", payload)
	c.Writer.Flush()
}

// devopsSSEPayload SSE 帧载荷。
type devopsSSEPayload struct {
	SessionID string `json:"sessionId"`
	Kind      string `json:"kind"`
	Data      string `json:"data,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
}

// ---- service ----

// relayOrError hub 未装配的统一 503。
func (s *Service) relayOrError() (*devopssessions.Hub, error) {
	if s.svcCtx == nil || s.svcCtx.DevopsSessions == nil {
		return nil, errDevopsRelayNotConfigured
	}
	return s.svcCtx.DevopsSessions, nil
}

// resolverOrError 会话解析缺位（装配不完整）。
func (s *Service) resolverOrError() (devopssessions.SessionResolver, error) {
	if s.svcCtx == nil || s.svcCtx.AgentSessionResolver == nil {
		return nil, errors.New("session resolver unavailable")
	}
	return s.svcCtx.AgentSessionResolver, nil
}

// devopsOperator 当前登录操作者（server 鉴权身份；匿名上下文回空串）。
func devopsOperator(ctx context.Context) string {
	if u, err := utils.CurrentUsername(ctx); err == nil {
		return u
	}
	return ""
}

// DevopsSessionOpen 经 agent 建立代理会话（scope fail-closed + open 审计）。
func (s *Service) DevopsSessionOpen(ctx context.Context, req *DevopsSessionOpenRequest) (*DevopsSessionOpenResponse, error) {
	agentID := strings.TrimSpace(req.AgentID)
	host := strings.TrimSpace(req.Host)
	protocolName := strings.TrimSpace(req.Protocol)
	if agentID == "" {
		return nil, errorx.NewBadRequest("agentId is required")
	}
	if host == "" {
		return nil, errorx.NewBadRequest("host is required")
	}
	if protocolName == "" {
		return nil, errorx.NewBadRequest("protocol is required")
	}
	if req.Port <= 0 {
		return nil, errorx.NewBadRequest("port must be positive")
	}
	// 跨 scope 动作拒绝（AGENT-001 同口径 fail-closed）。
	if err := NewAgentService(s.svcCtx).requireAgentScope(ctx, agentID); err != nil {
		return nil, err
	}
	relay, err := s.relayOrError()
	if err != nil {
		return nil, err
	}
	resolver, err := s.resolverOrError()
	if err != nil {
		return nil, err
	}
	operator := devopsOperator(ctx)
	scope := svc.GameScopeFromContext(ctx)
	sess, err := relay.Open(ctx, resolver, agentID, &opsv1.DevopsSessionOpenRequest{
		Operator: operator,
		Protocol: protocolName,
		Host:     host,
		Port:     req.Port,
		Cols:     req.Cols,
		Rows:     req.Rows,
	}, operator, scope.GameID, scope.Env)
	if err != nil {
		recordOpsAudit(ctx, s.svcCtx, audit.EventDevopsSessionOpen, agentID, "failed", map[string]interface{}{
			"host": host, "port": req.Port, "protocol": protocolName, "error": err.Error(),
		})
		return nil, err
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventDevopsSessionOpen, sess.SessionID, "ok", map[string]interface{}{
		"agentId": agentID, "host": host, "port": req.Port,
		"protocol": protocolName, "operator": operator,
	})
	return &DevopsSessionOpenResponse{
		SessionID: sess.SessionID,
		AgentID:   sess.AgentID,
		Protocol:  sess.Protocol,
		Host:      sess.Host,
		Port:      sess.Port,
		Channel:   sess.Channel,
	}, nil
}

// DevopsSessionsList 当前 scope 可见的代理会话（scope 全空 = 不限）。
func (s *Service) DevopsSessionsList(ctx context.Context) (*DevopsSessionListResponse, error) {
	relay, err := s.relayOrError()
	if err != nil {
		return nil, err
	}
	scope := svc.GameScopeFromContext(ctx)
	out := &DevopsSessionListResponse{Sessions: []DevopsSessionItem{}}
	for _, sess := range relay.List() {
		if scope.GameID != "" && sess.GameID != scope.GameID {
			continue
		}
		if scope.Env != "" && sess.Env != scope.Env {
			continue
		}
		out.Sessions = append(out.Sessions, DevopsSessionItem{
			SessionID: sess.SessionID,
			AgentID:   sess.AgentID,
			Operator:  sess.Operator,
			Protocol:  sess.Protocol,
			Host:      sess.Host,
			Port:      sess.Port,
			Channel:   sess.Channel,
			CreatedAt: sess.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return out, nil
}

// lookupScopedSession 取会话并做属主 scope 校验（会话绑定开单时的 agent
// 属主；input/resize/close/stream 同口径 fail-closed）。
func (s *Service) lookupScopedSession(ctx context.Context, sessionID string) (*devopssessions.Session, error) {
	relay, err := s.relayOrError()
	if err != nil {
		return nil, err
	}
	sess, ok := relay.Get(sessionID)
	if !ok {
		return nil, errorx.NewNotFound("devops session not found: " + sessionID)
	}
	if err := NewAgentService(s.svcCtx).requireAgentScope(ctx, sess.AgentID); err != nil {
		return nil, err
	}
	return sess, nil
}

// DevopsSessionStreamAuthorize SSE 流前置授权（连接建立前，404/403 走
// JSON 契约；流开始后不再回 JSON 错误帧）。
func (s *Service) DevopsSessionStreamAuthorize(ctx context.Context, sessionID string) error {
	_, err := s.lookupScopedSession(ctx, sessionID)
	return err
}

// AttachDevopsSessionStream 订阅会话事件流（回放 + 实时；会话已收束回 false）。
func (s *Service) AttachDevopsSessionStream(sessionID string) (*devopssessions.Subscription, bool) {
	relay, err := s.relayOrError()
	if err != nil {
		return nil, false
	}
	return relay.AttachConsumer(sessionID)
}

// DetachDevopsSessionStream SSE 断开退订。
func (s *Service) DetachDevopsSessionStream(sessionID string, sub *devopssessions.Subscription) {
	relay, err := s.relayOrError()
	if err != nil {
		return
	}
	relay.DetachConsumer(sessionID, sub)
}

// DevopsSessionInput 下行键盘数据（背压 → 429 session_busy）。
func (s *Service) DevopsSessionInput(ctx context.Context, sessionID, data string) error {
	if _, err := s.lookupScopedSession(ctx, sessionID); err != nil {
		return err
	}
	relay, err := s.relayOrError()
	if err != nil {
		return err
	}
	resolver, err := s.resolverOrError()
	if err != nil {
		return err
	}
	err = relay.Input(ctx, resolver, sessionID, []byte(data))
	return mapDevopsRelayError(err)
}

// DevopsSessionResize 下行窗口调整。
func (s *Service) DevopsSessionResize(ctx context.Context, sessionID string, cols, rows int32) error {
	if _, err := s.lookupScopedSession(ctx, sessionID); err != nil {
		return err
	}
	relay, err := s.relayOrError()
	if err != nil {
		return err
	}
	resolver, err := s.resolverOrError()
	if err != nil {
		return err
	}
	err = relay.Resize(ctx, resolver, sessionID, cols, rows)
	return mapDevopsRelayError(err)
}

// DevopsSessionClose 主动断开（close 审计留痕）。
func (s *Service) DevopsSessionClose(ctx context.Context, sessionID, reason string) error {
	sess, err := s.lookupScopedSession(ctx, sessionID)
	if err != nil {
		return err
	}
	relay, err := s.relayOrError()
	if err != nil {
		return err
	}
	resolver, err := s.resolverOrError()
	if err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		reason = "panel close"
	}
	err = relay.Close(ctx, resolver, sessionID, reason)
	recordOpsAudit(ctx, s.svcCtx, audit.EventDevopsSessionClose, sessionID, outcomeOf(err), map[string]interface{}{
		"agentId": sess.AgentID, "operator": sess.Operator, "reason": reason,
	})
	return mapDevopsRelayError(err)
}

// mapDevopsRelayError 中继错误 → REST 契约：未知会话 404、背压 429、
// 其余原样透传（agent 拒绝文案即错误消息）。
func mapDevopsRelayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, devopssessions.ErrSessionNotFound):
		return errorx.NewNotFound(err.Error())
	case errors.Is(err, devopssessions.ErrSessionBusy):
		return &errorx.CodeError{
			Code:       http.StatusTooManyRequests,
			Message:    "session input queue is full, retry later",
			StableCode: "session_busy",
		}
	default:
		return errorx.NewBadRequest(err.Error())
	}
}

// outcomeOf 审计 outcome 归一。
func outcomeOf(err error) string {
	if err == nil {
		return "ok"
	}
	return "failed"
}
