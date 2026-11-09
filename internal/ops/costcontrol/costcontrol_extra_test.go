package costcontrol

// 成本控制补测：价格目录 provider 未命中、磁盘附加费、Summary 的未来时间
// 钳制与未知规格不计价、闲置策略默认值/归一化、闲置评估的默认指标源/空
// agent/时长钳制/批量判定，预算评估的 nil/零预算/默认阈值/通知分发，以及
// 按域聚合的未知规格跳过与时间钳制。
//
// 诚实跳过：
//   - Evaluate 的 low == 0 防御臂（65-66）：窗口样本为空已在 len(cpus)==0
//     提前返回，循环后 low == len(cpus) > 0 恒成立，不可达。

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newCostExtraDB(t *testing.T) (*gorm.DB, *model.CostBudgetModel) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	return db, model.NewCostBudgetModel(db)
}

// 价格目录：provider 未命中（29）。
func TestRate_ProviderMiss(t *testing.T) {
	r, ok := Rate("unknown-cloud", "cx21")
	assert.False(t, ok)
	assert.Equal(t, 0.0, r)
}

// 估算：附加系统盘按 GB 折算小时价（72）。
func TestComputeEstimate_DiskCost(t *testing.T) {
	est, err := ComputeEstimate(EstimateInput{Provider: "hetzner", InstanceType: "cx21", Count: 1, Hours: 1, DiskSizeGB: 720})
	require.NoError(t, err)
	assert.InDelta(t, 0.08, est.DiskPerHour, 1e-9, "720GB * 0.08 / 720h = 0.08/h")
	assert.InDelta(t, 0.006+0.08, est.TotalUSD, 1e-9)
}

// Summary：未来创建时间钳为 0 时长（119）、未知规格不计价（123）。
func TestBuildSummary_ClampsAndUnknown(t *testing.T) {
	now := time.Now()
	runs := []model.ProvisionRun{
		{RunID: "future", Provider: "hetzner", CreatedAt: now.Add(time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"cx21"}`), AgentIDsJSON: model.JSON(`["a1"]`)},
		{RunID: "unknown-type", Provider: "hetzner", CreatedAt: now.Add(-time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"ghost.type"}`), AgentIDsJSON: model.JSON(`["a2"]`)},
	}
	sum := BuildSummary(runs, nil, nil, now)
	require.Len(t, sum.Nodes, 2)
	assert.Equal(t, 2, sum.TotalNodes)
	assert.Equal(t, 0.0, sum.TotalUSD, "未落地的时长与未知规格都不计价")
	byAgent := map[string]NodeCost{}
	for _, n := range sum.Nodes {
		byAgent[n.AgentID] = n
	}
	assert.InDelta(t, 0.0, byAgent["a1"].UptimeHours, 1e-9, "未来创建时间钳为 0")
	assert.Equal(t, 0.0, byAgent["a2"].CostUSD, "未知规格不计价")
}

// 策略：安全默认值（172）与 Normalize 补零（178/181）。
func TestDefaultIdlePolicyAndNormalize(t *testing.T) {
	def := DefaultIdlePolicy()
	assert.Equal(t, 24.0, def.IdleHours)
	assert.Equal(t, 5.0, def.CPUThreshold)
	require.NoError(t, def.Validate())

	p := IdlePolicy{}
	p.Normalize()
	assert.Equal(t, 24.0, p.IdleHours)
	assert.Equal(t, 5.0, p.CPUThreshold)

	p = IdlePolicy{IdleHours: 4}
	p.Normalize()
	assert.Equal(t, 4.0, p.IdleHours, "已设置的阈值不被覆盖")
	assert.Equal(t, 5.0, p.CPUThreshold)
}

// 评估器：nil 指标源注入默认函数（28）→ 视为无样本。
func TestIdleEvaluator_DefaultMetrics(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{}, nil)
	st := eval.Evaluate("a1", time.Now())
	assert.False(t, st.Idle)
	assert.Contains(t, st.Reason, "no recent")
}

// 评估器：空 agentID（46）与 Metrics 为 nil 的裸构造（49）。
func TestIdleEvaluator_EmptyAgentAndNilMetrics(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1}, func(string, time.Time) ([]float64, bool) {
		return []float64{1}, true
	})
	st := eval.Evaluate("", time.Now())
	assert.False(t, st.Idle)
	assert.Empty(t, st.Reason)

	bare := &IdleEvaluator{}
	st = bare.Evaluate("a1", time.Now())
	assert.False(t, st.Idle)
	assert.Contains(t, st.Reason, "metrics unavailable")
}

// 评估器：估算时长超出策略阈值时钳制（69）。
func TestIdleEvaluator_ClampsIdleHours(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1, CPUThreshold: 5}, func(string, time.Time) ([]float64, bool) {
		return []float64{0, 0, 0, 0, 0}, true // 5 * 0.25h = 1.25h > 1h
	})
	st := eval.Evaluate("a1", time.Now())
	require.True(t, st.Idle)
	assert.Equal(t, 1.0, st.IdleHours, "估算时长钳到策略阈值")
}

// 批量判定：按 agent 逐个评估（80-84）。
func TestIdleEvaluator_IdleSummary(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1, CPUThreshold: 5}, func(id string, _ time.Time) ([]float64, bool) {
		if id == "a1" {
			return []float64{1, 2}, true
		}
		return []float64{9}, true
	})
	out := eval.IdleSummary([]string{"a1", "a2"}, time.Now())
	require.Len(t, out, 2)
	assert.True(t, out["a1"].Idle)
	assert.False(t, out["a2"].Idle)
}

// 预算：nil 接收器与无 Store 均返回 (nil, nil)（111）。
func TestBudgetEvaluator_NilAndNoStore(t *testing.T) {
	ctx := context.Background()
	var nilEval *BudgetEvaluator
	alert, err := nilEval.Evaluate(ctx, "g", "e", 10)
	require.NoError(t, err)
	assert.Nil(t, alert)

	alert, err = (&BudgetEvaluator{}).Evaluate(ctx, "g", "e", 10)
	require.NoError(t, err)
	assert.Nil(t, alert)
}

// 预算：已配置但月预算为 0 → 不评估（122）。
func TestBudgetEvaluator_ZeroBudget(t *testing.T) {
	_, m := newCostExtraDB(t)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e"}))
	alert, err := (&BudgetEvaluator{Store: m}).Evaluate(context.Background(), "g", "e", 90)
	require.NoError(t, err)
	assert.Nil(t, alert)
}

// 预算：未设置阈值时默认 80（127）。
func TestBudgetEvaluator_DefaultThreshold(t *testing.T) {
	_, m := newCostExtraDB(t)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e", MonthlyUSD: 100}))
	alert, err := (&BudgetEvaluator{Store: m}).Evaluate(context.Background(), "g", "e", 85)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, 80.0, alert.ThresholdPct)
	assert.True(t, alert.OverThreshold)
}

// 预算：超阈值且配置通知服务时触发分发（134-147）。
func TestBudgetEvaluator_DispatchNotify(t *testing.T) {
	_, m := newCostExtraDB(t)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e", MonthlyUSD: 100, NotifyChannelsJSON: `["in_app"]`}))
	eval := &BudgetEvaluator{Store: m, ThresholdPct: 80, Notify: notify.New(nil, nil)}
	alert, err := eval.Evaluate(context.Background(), "g", "e", 90)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.True(t, alert.OverThreshold)
	assert.Equal(t, "g", alert.GameID)
}

// 聚合：未知规格跳过（162）、未来创建时间贡献为 0（166）。
func TestAggregateByScope_SkipsAndClamps(t *testing.T) {
	now := time.Now()
	runs := []model.ProvisionRun{
		{RunID: "unknown", Provider: "hetzner", GameID: "g", Env: "e", CreatedAt: now.Add(-time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"ghost.type"}`)},
		{RunID: "future", Provider: "hetzner", GameID: "g", Env: "future", CreatedAt: now.Add(time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"cx21"}`)},
		{RunID: "valid", Provider: "hetzner", GameID: "g", Env: "e", CreatedAt: now.Add(-2 * time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"cx21"}`)},
	}
	out := AggregateByScope(runs, now)
	assert.InDelta(t, 2*0.006, out["g/e"], 1e-9)
	assert.NotContains(t, out, "g/unknown", "未知规格不入账")
	require.Contains(t, out, "g/future")
	assert.InDelta(t, 0.0, out["g/future"], 1e-9)
}
