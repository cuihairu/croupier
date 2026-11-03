package costcontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	notify "github.com/cuihairu/croupier/internal/service/notify"
)

// IdleEvaluator 评估单机闲置状态（低负载判定）。
type IdleEvaluator struct {
	Policy IdlePolicy
	// Metrics 查询单机最近 CPU 使用率（nil=无数据源，全部视为未知）。
	Metrics func(agentID string, since time.Time) ([]float64, bool)
	// Now 注入时钟（测试用）。
	Now func() time.Time
}

// NewIdleEvaluator 构造闲置评估器（策略归一化）。
func NewIdleEvaluator(policy IdlePolicy, metrics func(string, time.Time) ([]float64, bool)) *IdleEvaluator {
	policy.Normalize()
	if metrics == nil {
		metrics = func(string, time.Time) ([]float64, bool) { return nil, false }
	}
	return &IdleEvaluator{Policy: policy, Metrics: metrics, Now: time.Now}
}

// IdleState 是单机闲置判定结果。
type IdleState struct {
	AgentID    string  `json:"agentId"`
	Idle       bool    `json:"idle"`
	IdleHours  float64 `json:"idleHours"`
	Reason     string  `json:"reason,omitempty"`
	Actionable bool    `json:"actionable"`
}

// Evaluate 对单机判断是否满足闲置（CPU 低于阈值持续 >= IdleHours）。
// 无数据源/无近期样本视为未知（Idle=false，不误判）。
func (e *IdleEvaluator) Evaluate(agentID string, latestAt time.Time) IdleState {
	if agentID == "" {
		return IdleState{AgentID: agentID}
	}
	if e.Metrics == nil {
		return IdleState{AgentID: agentID, Reason: "metrics unavailable"}
	}
	window := time.Duration(e.Policy.IdleHours * float64(time.Hour))
	cpus, ok := e.Metrics(agentID, latestAt.Add(-window))
	if !ok || len(cpus) == 0 {
		return IdleState{AgentID: agentID, Reason: "no recent samples"}
	}
	// 窗口内全部样本低于阈值才判闲置（保守：任何超阈样本取消）。
	low := 0
	for _, v := range cpus {
		if v >= e.Policy.CPUThreshold {
			return IdleState{AgentID: agentID, Idle: false, Reason: "cpu above threshold"}
		}
		low++
	}
	if low == 0 {
		return IdleState{AgentID: agentID, Idle: false, Reason: "cpu above threshold"}
	}
	idleHours := float64(low) * 0.25 // 样本间隔按 15s 上报折算（仅估算）
	if idleHours > e.Policy.IdleHours {
		idleHours = e.Policy.IdleHours
	}
	return IdleState{
		AgentID: agentID, Idle: true, IdleHours: idleHours,
		Actionable: e.Policy.Enabled && e.Policy.AutoDestroy,
		Reason:     fmt.Sprintf("cpu < %.1f%% for %d samples", e.Policy.CPUThreshold, low),
	}
}

// IdleSummary 批量闲置判定。
func (e *IdleEvaluator) IdleSummary(agentIDs []string, latestAt time.Time) map[string]IdleState {
	out := make(map[string]IdleState, len(agentIDs))
	for _, id := range agentIDs {
		out[id] = e.Evaluate(id, latestAt)
	}
	return out
}

// BudgetAlert 是预算告警载荷。
type BudgetAlert struct {
	GameID        string  `json:"gameId"`
	Env           string  `json:"env"`
	MonthlyUSD    float64 `json:"monthlyUsd"`
	SpentUSD      float64 `json:"spentUsd"`
	ThresholdPct  float64 `json:"thresholdPct"`
	OverThreshold bool    `json:"overThreshold"`
}

// BudgetEvaluator 评估月度预算使用。
type BudgetEvaluator struct {
	// Store 读取预算配置（nil=无配置）。
	Store *model.CostBudgetModel
	// Notify 分发通知（nil=跳过）。
	Notify *notify.Service
	// ThresholdPct 触发阈值（默认 80）。
	ThresholdPct float64
}

// Evaluate 对给定 scope 评估预算（spent 由调用方按 game/env 聚合传入）。
// 超过阈值时触发通知；未配置预算返回 nil（非故障）。
func (e *BudgetEvaluator) Evaluate(ctx context.Context, gameID, env string, spentUSD float64) (*BudgetAlert, error) {
	if e == nil || e.Store == nil {
		return nil, nil
	}
	b, err := e.Store.Get(ctx, gameID, env)
	if err != nil {
		return nil, nil
	}
	if b == nil || b.MonthlyUSD <= 0 {
		return nil, nil
	}
	pct := spentUSD / b.MonthlyUSD * 100
	threshold := e.ThresholdPct
	if threshold <= 0 {
		threshold = 80
	}
	alert := &BudgetAlert{
		GameID: gameID, Env: env, MonthlyUSD: b.MonthlyUSD,
		SpentUSD: spentUSD, ThresholdPct: threshold, OverThreshold: pct >= threshold,
	}
	if alert.OverThreshold && e.Notify != nil {
		var channels []string
		_ = json.Unmarshal([]byte(b.NotifyChannelsJSON), &channels)
		_ = channels
		e.Notify.Dispatch(ctx, notify.Event{
			Type:     "cost.budget",
			Title:    "[预算告警] " + gameID + "/" + env,
			Message:  fmt.Sprintf("本月已用 $%.2f / 预算 $%.2f (%.0f%%)", spentUSD, b.MonthlyUSD, pct),
			Priority: "warning",
			Data: map[string]interface{}{
				"gameId": gameID, "env": env, "spentUsd": spentUSD,
				"monthlyUsd": b.MonthlyUSD, "pct": pct,
			},
		})
	}
	return alert, nil
}

// AggregateByScope 按 (game/env) 聚合 run 成本。
func AggregateByScope(runs []model.ProvisionRun, now time.Time) map[string]float64 {
	out := map[string]float64{}
	for i := range runs {
		r := &runs[i]
		var spec struct {
			InstanceType string `json:"instanceType"`
		}
		_ = json.Unmarshal(r.SpecJSON, &spec)
		rate, known := Rate(r.Provider, spec.InstanceType)
		if !known {
			continue
		}
		uptime := now.Sub(r.CreatedAt).Hours()
		if uptime < 0 {
			uptime = 0
		}
		key := r.GameID + "/" + r.Env
		out[key] += rate * uptime
	}
	return out
}
