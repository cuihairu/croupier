// Package alertrule 实现主机指标阈值告警规则引擎。
//
// 评估挂在 Agent 指标上报路径（MetricsStore.Evaluator 钩子）：
// 每次上报提取规则声明的指标路径当前值 → 比较 → 连续命中达 forCount
// 且不在冷却期 → 写 Alert（alerts 表）+ notify 分发（站内信/钉钉/webhook）。
//
// 指标路径：
//
//	cpu.usagePercent          CPU 使用率（0-100）
//	memory.usagePercent       内存使用率（0-100）
//	memory.usedBytes          内存已用字节
//	disk.<mount>.usedPercent  指定挂载点磁盘使用率
//	disk.<mount>.usedBytes    指定挂载点磁盘已用字节
//	custom.<key>              Agent 自定义指标
package alertrule

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// Evaluator 评估规则并触发告警。
type Evaluator struct {
	rules  *model.AlertRuleModel
	alerts *model.AlertModel
	notify *notify.Service
}

// New creates an evaluator. alerts/notify 可为 nil（仅记日志）。
func New(rules *model.AlertRuleModel, alerts *model.AlertModel, notifySvc *notify.Service) *Evaluator {
	return &Evaluator{rules: rules, alerts: alerts, notify: notifySvc}
}

// EvaluateAgent 对一次 agent 上报评估全部启用规则。
func (e *Evaluator) EvaluateAgent(ctx context.Context, agentID string, report *opsv1.MetricsReport) {
	if e == nil || e.rules == nil || agentID == "" || report == nil {
		return
	}
	rules, err := e.rules.List(ctx, model.ListAlertRulesOptions{Enabled: boolPtr(true)})
	if err != nil {
		slog.WarnContext(ctx, "alertrule: list rules failed", "error", err)
		return
	}
	for i := range rules {
		e.evaluateRule(ctx, &rules[i], agentID, report)
	}
}

func (e *Evaluator) evaluateRule(ctx context.Context, rule *model.AlertRule, agentID string, report *opsv1.MetricsReport) {
	// agent 过滤。
	if rule.AgentFilter != "" && rule.AgentFilter != agentID {
		return
	}
	value, ok := ExtractMetric(rule.Metric, report)
	if !ok {
		return // 指标缺失（如该挂载点不存在）：视为未命中
	}
	hit := Compare(value, rule.Operator, rule.Threshold)

	forCount := rule.ForCount
	if forCount < 1 {
		forCount = 1
	}
	if hit {
		rule.HitCount++
		// 连续命中计数必须跨上报持久（评估是无状态遍历）。
		_ = e.rules.Update(ctx, rule.ID, map[string]interface{}{"hit_count": rule.HitCount})
	} else {
		if rule.HitCount > 0 {
			// 未命中即重置计数。
			e.resetHits(ctx, rule)
		}
		return
	}
	if rule.HitCount < forCount {
		return
	}

	// 冷却判定。
	cooldown := time.Duration(rule.CooldownSeconds) * time.Second
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	if rule.LastFiredAt != nil && time.Since(*rule.LastFiredAt) < cooldown {
		return
	}

	e.fire(ctx, rule, agentID, value)
}

func (e *Evaluator) resetHits(ctx context.Context, rule *model.AlertRule) {
	rule.HitCount = 0
	_ = e.rules.Update(ctx, rule.ID, map[string]interface{}{"hit_count": 0})
}

func (e *Evaluator) fire(ctx context.Context, rule *model.AlertRule, agentID string, value float64) {
	now := time.Now().UTC()
	rule.HitCount = 0
	rule.LastFiredAt = &now
	_ = e.rules.Update(ctx, rule.ID, map[string]interface{}{"hit_count": 0, "last_fired_at": now})

	msg := fmt.Sprintf("%s: agent=%s %s %s %.2f（当前 %.2f）",
		rule.Name, agentID, rule.Metric, describeOperator(rule.Operator), rule.Threshold, value)

	alert := &model.Alert{
		AlertID: fmt.Sprintf("rule-%d-%d", rule.ID, now.Unix()),
		Type:    "metric." + rule.Metric,
		Level:   rule.Level,
		Message: msg,
		Source:  "rule:" + rule.Name,
		Status:  "firing",
		Details: map[string]interface{}{
			"ruleId": rule.ID, "agentId": agentID, "metric": rule.Metric,
			"operator": rule.Operator, "threshold": rule.Threshold, "value": value,
		},
		CreatedBy: "alertrule",
	}
	if e.alerts != nil {
		if err := e.alerts.Create(ctx, alert); err != nil {
			slog.WarnContext(ctx, "alertrule: create alert failed", "error", err)
		}
	}
	if e.notify != nil {
		e.notify.Dispatch(ctx, notify.Event{
			Type:     "alert.fired",
			Title:    "[告警] " + rule.Name,
			Message:  msg,
			Priority: levelToPriority(rule.Level),
			Data: map[string]interface{}{
				"ruleId": rule.ID, "agentId": agentID, "level": rule.Level,
			},
		})
	}
	slog.Warn("alertrule: fired", "rule", rule.Name, "agent", agentID, "value", value)
}

// ExtractMetricFromName 校验指标路径形态（不需要 report）。
// 合法形态：cpu.usagePercent / memory.usagePercent / memory.usedBytes /
// disk.<mount>.<usedPercent|usedBytes> / custom.<key>
func ExtractMetricFromName(path string) (string, error) {
	path = strings.TrimSpace(path)
	switch {
	case path == "cpu.usagePercent", path == "memory.usagePercent", path == "memory.usedBytes":
		return path, nil
	case strings.HasPrefix(path, "disk."):
		rest := strings.TrimPrefix(path, "disk.")
		field, mount, ok := splitDiskPath(rest)
		if !ok || mount == "" {
			return "", fmt.Errorf("非法磁盘指标 %q（期望 disk.<挂载点>.usedPercent|usedBytes）", path)
		}
		if field != "usedPercent" && field != "usedBytes" {
			return "", fmt.Errorf("非法磁盘指标字段 %q（仅支持 usedPercent/usedBytes）", field)
		}
		return path, nil
	case strings.HasPrefix(path, "custom."):
		key := strings.TrimPrefix(path, "custom.")
		if key == "" {
			return "", fmt.Errorf("custom 指标缺少 key")
		}
		return path, nil
	default:
		return "", fmt.Errorf("非法指标路径 %q（支持 cpu.usagePercent / memory.* / disk.* / custom.*）", path)
	}
}

// ExtractMetric 从上报中提取指标路径的当前值。
func ExtractMetric(path string, report *opsv1.MetricsReport) (float64, bool) {
	path = strings.TrimSpace(path)
	switch {
	case path == "cpu.usagePercent":
		return report.GetCpu().GetUsagePercent(), report.GetCpu() != nil
	case path == "memory.usagePercent":
		m := report.GetMemory()
		if m == nil || m.GetTotalBytes() == 0 {
			return 0, false
		}
		return float64(m.GetUsedBytes()) / float64(m.GetTotalBytes()) * 100, true
	case path == "memory.usedBytes":
		m := report.GetMemory()
		return float64(m.GetUsedBytes()), m != nil
	case strings.HasPrefix(path, "disk."):
		rest := strings.TrimPrefix(path, "disk.")
		field, mount, found := splitDiskPath(rest)
		if !found {
			return 0, false
		}
		for _, d := range report.GetDisks() {
			if d.GetMountPoint() != mount {
				continue
			}
			switch field {
			case "usedPercent":
				return d.GetUsagePercent(), true
			case "usedBytes":
				return float64(d.GetUsedBytes()), true
			}
		}
		return 0, false
	case strings.HasPrefix(path, "custom."):
		key := strings.TrimPrefix(path, "custom.")
		v, ok := report.GetCustom()[key]
		return v, ok
	default:
		return 0, false
	}
}

// splitDiskPath 解析 "<mount>.<field>"（mount 可含 / 与 -）。
func splitDiskPath(rest string) (field, mount string, ok bool) {
	idx := strings.LastIndex(rest, ".")
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", false
	}
	return rest[idx+1:], rest[:idx], true
}

// Compare 比较 value 与 threshold。
func Compare(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

func describeOperator(op string) string {
	switch op {
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	}
	return op
}

func levelToPriority(level string) string {
	switch level {
	case model.AlertRuleLevelCritical:
		return "urgent"
	case model.AlertRuleLevelWarning:
		return "high"
	default:
		return "normal"
	}
}

func boolPtr(b bool) *bool { return &b }
