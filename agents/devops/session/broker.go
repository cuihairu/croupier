package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/core/execlog"
)

// Audit 审计通道（audit-first 闸的会话级应用，agent-core §7.3）：Admit 是
// 放行凭据（记录未持久化即拒绝连接），Record 是结果留痕。生产装配见
// runner（execlog Gate+Logger 适配），测试注入 fake。
type Audit interface {
	Admit(rec *execlog.Record) error
	Record(rec *execlog.Record) error
}

// 会话审计动作名（execlog 真值里会话面与命令执行面按 action 区分）。
const (
	actionSessionOpen = "session.open"
	actionSessionEnd  = "session.end"
)

// ExecAudit execlog 适配（生产装配）。
type ExecAudit struct {
	Gate *execlog.Gate
	Log  *execlog.Logger
}

func (a *ExecAudit) Admit(rec *execlog.Record) error  { return a.Gate.Admit(rec) }
func (a *ExecAudit) Record(rec *execlog.Record) error { return a.Log.Record(rec) }

// Broker 会话建立链（顺序不可倒，§7.5）：请求校验 → 白名单 → 审计落库
// （admitted 放行凭据）→ Provider Open；任一步失败即拒绝。审计 scope 取
// Broker 身份（agent 注册标签），不取请求方——请求方可疑，身份面以配置
// 为准。
type Broker struct {
	allow     *Allowlist
	audit     Audit
	provider  Target
	recorder  *Recorder // nil = 不落会话存档（审计仍有 execlog 真值）
	agentType string
	instance  string
	gameID    string
	env       string
	now       func() time.Time // 测试注入
}

// NewBroker 装配会话建立链。
func NewBroker(allow *Allowlist, audit Audit, provider Target, rec *Recorder,
	agentType, instance, gameID, env string) *Broker {
	return &Broker{
		allow:     allow,
		audit:     audit,
		provider:  provider,
		recorder:  rec,
		agentType: agentType,
		instance:  instance,
		gameID:    gameID,
		env:       env,
		now:       time.Now,
	}
}

// OpenSession 走完整建立链；返回的 Session Close 时自动落会话结束存档。
func (b *Broker) OpenSession(ctx context.Context, req SessionRequest) (Session, error) {
	if b == nil || b.allow == nil || b.audit == nil || b.provider == nil {
		return nil, fmt.Errorf("session: broker not wired")
	}
	req.Operator = strings.TrimSpace(req.Operator)
	req.Host = strings.TrimSpace(req.Host)
	if req.Operator == "" {
		return nil, fmt.Errorf("session: operator is required")
	}
	if !req.Protocol.Valid() {
		return nil, fmt.Errorf("session: protocol %q not supported", req.Protocol)
	}
	if req.Host == "" || req.Port <= 0 || req.Port > 65535 {
		return nil, fmt.Errorf("session: host/port invalid")
	}
	if req.SessionID == "" {
		req.SessionID = newSessionID()
	}

	// 1) 白名单前置（白名单外拒绝也要留痕：谁试图进哪台）。
	if err := b.allow.Check(req.Host, req.Port, req.Protocol); err != nil {
		b.result(req, execlog.StatusFailure, err.Error(), 0)
		return nil, err
	}

	// 2) 审计落库 = 放行凭据（无审计通道不得开连接）。
	started := b.now()
	params, _ := json.Marshal(map[string]interface{}{
		"protocol": string(req.Protocol),
		"host":     req.Host,
		"port":     req.Port,
		"channel":  b.provider.Name(),
	})
	if err := b.audit.Admit(&execlog.Record{
		TaskID:        req.SessionID,
		Operator:      req.Operator,
		AgentType:     b.agentType,
		AgentInstance: b.instance,
		Action:        "session.open",
		Params:        params,
		Status:        execlog.StatusAdmitted, // execlog.Gate.Admit 会再置同值，测试 fake 直录
		GameID:        b.gameID,
		Env:           b.env,
		TsUnixMs:      started.UnixMilli(),
	}); err != nil {
		return nil, fmt.Errorf("session: audit-first rejected (channel not persisted): %w", err)
	}

	// 3) Provider Open（协议通道）。
	sess, err := b.provider.Open(ctx, req)
	elapsed := b.now().Sub(started).Milliseconds()
	if err != nil {
		b.result(req, execlog.StatusFailure, err.Error(), elapsed)
		return nil, err
	}
	b.result(req, execlog.StatusSuccess, "", elapsed)

	// 4) 会话存档（起止元数据）。交互终端通道（SSH 等）的生命周期由 Hub
	// 收束——结束存档（EndStats 带回放档统计）与会话结束审计在 cleanup
	// 单点落，这里返回原生 Terminal 避免双重记账；非终端通道（guacd 批次
	// 前占位等）沿用 archivedSession 包装（Close 即结束存档）。
	meta := SessionMeta{
		SessionID:     req.SessionID,
		Protocol:      string(req.Protocol),
		Host:          req.Host,
		Port:          req.Port,
		Operator:      req.Operator,
		Channel:       b.provider.Name(),
		StartedUnixMs: started.UnixMilli(),
	}
	if term, ok := sess.(Terminal); ok {
		if b.recorder != nil {
			_ = b.recorder.Start(meta) // 存档失败不回滚已审计会话，落日志由调用方兜底
		}
		return term, nil
	}
	if b.recorder != nil {
		_ = b.recorder.Start(meta) // 存档失败不回滚已审计会话，落日志由调用方兜底
	}
	return &archivedSession{Session: sess, meta: meta, rec: b.recorder, broker: b}, nil
}

// SwapAllowlist 原子整体替换白名单（configsync 下发热生效；Check 读侧
// 永远看到完整一致的一份名单）。返回 (prev, cur) 条目数供日志。
func (b *Broker) SwapAllowlist(entries []config.AllowHost) (prev, cur int) {
	return b.allow.Swap(entries)
}

// Channel 会话通道名（存档与日志辨识用）。
func (b *Broker) Channel() string {
	if b == nil || b.provider == nil {
		return ""
	}
	return b.provider.Name()
}

// Refuse 建立链之外的拒绝留痕（准入上限等 Hub 侧闸）——谁试图进哪台、
// 为什么被拒，同样进审计（白名单拒绝在链内已留痕，此处补链外闸的洞）。
func (b *Broker) Refuse(req SessionRequest, reason string) {
	if b == nil || b.audit == nil {
		return
	}
	if req.SessionID == "" {
		req.SessionID = newSessionID()
	}
	params, _ := json.Marshal(map[string]interface{}{
		"protocol": string(req.Protocol),
		"host":     req.Host,
		"port":     req.Port,
	})
	_ = b.audit.Record(&execlog.Record{
		TaskID:        req.SessionID,
		Operator:      req.Operator,
		AgentType:     b.agentType,
		AgentInstance: b.instance,
		Action:        actionSessionOpen,
		Params:        params,
		Status:        execlog.StatusFailure,
		Summary:       reason,
		GameID:        b.gameID,
		Env:           b.env,
		TsUnixMs:      b.now().UnixMilli(),
	})
}

// RecordEnd 会话结束审计留痕（时长=堡垒机留痕四要素收口；Hub 断线清理
// 单点调用）。outcome ∈ closed|eof|error|cast-broken|no-terminal-channel|…
func (b *Broker) RecordEnd(sessionID, operator, outcome string, elapsedMs int64) {
	if b == nil || b.audit == nil {
		return
	}
	status := execlog.StatusSuccess
	if outcome == "error" || outcome == "cast-broken" {
		status = execlog.StatusFailure
	}
	_ = b.audit.Record(&execlog.Record{
		TaskID:        sessionID,
		Operator:      operator,
		AgentType:     b.agentType,
		AgentInstance: b.instance,
		Action:        actionSessionEnd,
		Status:        status,
		Summary:       outcome,
		DurationMs:    elapsedMs,
		GameID:        b.gameID,
		Env:           b.env,
		TsUnixMs:      b.now().UnixMilli(),
	})
}

// result 落一条结果记录（观测留痕，尽力而为——失败不阻断主流程）。
func (b *Broker) result(req SessionRequest, status execlog.Status, summary string, elapsedMs int64) {
	rec := &execlog.Record{
		TaskID:        req.SessionID,
		Operator:      req.Operator,
		AgentType:     b.agentType,
		AgentInstance: b.instance,
		Action:        actionSessionOpen,
		Status:        status,
		Summary:       summary,
		DurationMs:    elapsedMs,
		GameID:        b.gameID,
		Env:           b.env,
		TsUnixMs:      b.now().UnixMilli(),
	}
	_ = b.audit.Record(rec)
}

// archivedSession 包装会话：Close 时落会话结束存档（时长=堡垒机留痕四要素
// 的最后一项）。
type archivedSession struct {
	Session
	meta   SessionMeta
	rec    *Recorder
	broker *Broker
}

func (s *archivedSession) Close() error {
	err := s.Session.Close()
	outcome := "closed"
	if err != nil {
		outcome = "close-error"
	}
	if s.rec != nil {
		_ = s.rec.End(s.meta.SessionID, outcome)
	}
	return err
}
