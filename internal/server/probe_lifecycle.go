package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// 窗口→Alert 生命周期（review §12 R3 / §13.1）。
//
// 消费 probe.window 事件维护 Alert 行：unavailable → firing（幂等 upsert），
// recovered → resolved（更新既有行）。alertID 稳定派生（agent+target），
// 开/合落在同一行——与入站 webhook 的 alertID 唯一键去重语义同构，但生命周期
// 转换需要「已存在也更新状态」，因此不走 UpsertByAlertID（它去重不更新）。
//
// 失败明细序列（probe.detail 诊断环）在开窗时随 Alert details 永久落库
// （§13.1「开窗时最近 N 次序列随 Alert details 永久落库」）：明细面=内存诊断
// 环（30min 保留），长期留存=Alert 行 details（双通道语义，同既有
// FileTimeline 口径）。
const (
	// probeAlertKind 探测告警类型（Alert.Type，可 grep 的闭集标识）。
	probeAlertKind = "probe"
	// probeAlertSource 探测告警来源（Alert.Source）。
	probeAlertSource = "probe"
	// probeWindowRecentFailures 随 details 永久落库的最近失败明细条数
	// （§13.1：最近 N=5 次序列）。
	probeWindowRecentFailures = 5
)

// probeAlertID 探测告警的 alertID 派生（review §12 R3）：agent+target 唯一，
// 窗口开/合复用同 id——生命周期转换落在同一行。
func probeAlertID(agentID, target string) string {
	return "probe:" + agentID + ":" + target
}

// probeLifecycle 探测窗口事件的 Alert 生命周期消费者。
type probeLifecycle struct {
	alerts *model.AlertModel
	sink   *probeEventSink
	logger *slog.Logger
}

func newProbeLifecycle(alerts *model.AlertModel, sink *probeEventSink, logger *slog.Logger) *probeLifecycle {
	if logger == nil {
		logger = slog.Default()
	}
	return &probeLifecycle{alerts: alerts, sink: sink, logger: logger}
}

// onWindow 处理一条 probe.window 事件。alerts 为 nil（未装配）时静默跳过——
// 诊断环已先行留证，生命周期是消费面不是接收面，缺装配不报错。
func (l *probeLifecycle) onWindow(ctx context.Context, agentID string, ev *opsv1.AlertEvent) {
	if l == nil || l.alerts == nil || l.logger == nil {
		return
	}
	w := ev.GetWindow()
	if w == nil {
		l.logger.Warn("probe lifecycle: nil window payload", "agent_id", agentID)
		return
	}
	target := strings.TrimSpace(w.GetTarget())
	if target == "" {
		l.logger.Warn("probe lifecycle: window target required", "agent_id", agentID)
		return
	}
	details := l.buildDetails(agentID, ev)
	alertID := probeAlertID(agentID, target)

	switch strings.TrimSpace(w.GetWindowKind()) {
	case "unavailable":
		if err := l.transitionFiring(ctx, alertID, ev, w, details); err != nil {
			l.logger.Warn("probe lifecycle: firing transition failed",
				"agent_id", agentID, "target", target, "error", err)
		}
	case "recovered":
		if err := l.transitionResolved(ctx, alertID, ev, w, details); err != nil {
			l.logger.Warn("probe lifecycle: resolved transition failed",
				"agent_id", agentID, "target", target, "error", err)
		}
	default:
		l.logger.Warn("probe lifecycle: unsupported window_kind",
			"agent_id", agentID, "target", target, "window_kind", w.GetWindowKind())
	}
}

// transitionFiring unavailable → firing：不存在则建；已 resolved 则复活为新事件
// （status + message + details 刷到最新窗口）；firing 中重放 → 幂等不动（重连
// 重放不产生重复行）。
func (l *probeLifecycle) transitionFiring(ctx context.Context, alertID string, ev *opsv1.AlertEvent, w *opsv1.ProbeWindow, details map[string]interface{}) error {
	row, err := l.alerts.FindByAlertID(ctx, alertID)
	if err != nil {
		alert := newProbeWindowAlert(alertID, ev, w, details, "firing")
		return l.alerts.Create(ctx, alert)
	}
	if row.Status == "firing" {
		return nil
	}
	return l.alerts.UpdateLifecycle(ctx, row.AlertID, "firing", probeWindowMessage(w, "firing"), details)
}

// transitionResolved recovered → resolved：不存在（server 重启错过了开窗事件）
// 也落 resolved 行——事实优先，不丢闭窗事实；firing → resolved 转换并刷
// message + details（resolved 行自带完整窗口+明细序列，排障自包含）。
func (l *probeLifecycle) transitionResolved(ctx context.Context, alertID string, ev *opsv1.AlertEvent, w *opsv1.ProbeWindow, details map[string]interface{}) error {
	row, err := l.alerts.FindByAlertID(ctx, alertID)
	if err != nil {
		alert := newProbeWindowAlert(alertID, ev, w, details, "resolved")
		return l.alerts.Create(ctx, alert)
	}
	if row.Status == "resolved" {
		return nil
	}
	return l.alerts.UpdateLifecycle(ctx, row.AlertID, "resolved", probeWindowMessage(w, "resolved"), details)
}

// newProbeWindowAlert 组装探测窗口告警行。scope 打戳取事件载荷（与 capture
// 告警同口径：载荷自带 game/env，空 = 平台级）。
func newProbeWindowAlert(alertID string, ev *opsv1.AlertEvent, w *opsv1.ProbeWindow, details map[string]interface{}, status string) *model.Alert {
	return &model.Alert{
		AlertID:   alertID,
		Type:      probeAlertKind,
		Level:     "warning",
		Message:   probeWindowMessage(w, status),
		Source:    probeAlertSource,
		Status:    status,
		Details:   details,
		CreatedBy: "probe",
		// 4862：scope 打戳来自事件载荷（agent 侧注册会话与 payload 一致）。
		// 空 = 平台级告警（全局可见）。
		GameID: ev.GetGameId(),
		Env:    ev.GetEnv(),
	}
}

// probeWindowMessage 窗口告警文案：中文（同 evaluator/notify 服务端生成告警
// 的语言口径），状态与持续时长进文案，明细进 details。
func probeWindowMessage(w *opsv1.ProbeWindow, status string) string {
	target := w.GetTarget()
	switch status {
	case "resolved":
		return fmt.Sprintf("探针目标 %s 恢复可用（故障持续 %dms）", target, w.GetDurationMs())
	default:
		return fmt.Sprintf("探针目标 %s 探测不可用（%s）", target, w.GetSemantics())
	}
}

// buildDetails 组装 Alert details：窗口本体字段 + 最近 N 次失败明细。
func (l *probeLifecycle) buildDetails(agentID string, ev *opsv1.AlertEvent) map[string]interface{} {
	w := ev.GetWindow()
	details := map[string]interface{}{
		"kind":          "probe.window",
		"agentId":       agentID,
		"gameId":        ev.GetGameId(),
		"env":           ev.GetEnv(),
		"target":        w.GetTarget(),
		"semantics":     w.GetSemantics(),
		"windowKind":    w.GetWindowKind(),
		"startTsUnixMs": w.GetStartTsUnixMs(),
		"endTsUnixMs":   w.GetEndTsUnixMs(),
		"durationMs":    w.GetDurationMs(),
		"lastReason":    w.GetLastReason(),
	}
	if l != nil && l.sink != nil {
		details["recentFailures"] = l.recentFailures(agentID, w.GetTarget(), w.GetStartTsUnixMs())
	}
	return details
}

// recentFailures 取本窗口内最近 N 条失败明细（时间正序）。过滤口径：
// kind=probe.detail + target 匹配 + at >= 窗口起点（只看本窗口，历史窗口的
// 明细不串台）。
func (l *probeLifecycle) recentFailures(agentID, target string, startTs int64) []map[string]interface{} {
	if l == nil || l.sink == nil {
		return nil
	}
	var out []map[string]interface{}
	for _, ev := range l.sink.recent(agentID, 0) {
		if ev == nil || ev.GetKind() != "probe.detail" || ev.GetDetail() == nil {
			continue
		}
		d := ev.GetDetail()
		if d.GetTarget() != target || d.GetAtTsUnixMs() < startTs {
			continue
		}
		out = append(out, map[string]interface{}{
			"target":              d.GetTarget(),
			"err":                 d.GetErr(),
			"atTsUnixMs":          d.GetAtTsUnixMs(),
			"latencyMs":           d.GetLatencyMs(),
			"consecutiveFailures": d.GetConsecutiveFailures(),
			"windowOpen":          d.GetWindowOpen(),
			"details":             d.GetDetails(),
		})
	}
	if len(out) > probeWindowRecentFailures {
		out = out[len(out)-probeWindowRecentFailures:]
	}
	return out
}
