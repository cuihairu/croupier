package costcontrol

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestComputeEstimate(t *testing.T) {
	est, err := ComputeEstimate(EstimateInput{Provider: "hetzner", InstanceType: "cx21", Count: 2, Hours: 10})
	require.NoError(t, err)
	assert.Equal(t, 0.006*2*10, est.TotalUSD)
	assert.True(t, est.KnownRate)
	assert.Equal(t, 10.0, est.Hours)
}

func TestComputeEstimate_DefaultsHours(t *testing.T) {
	est, err := ComputeEstimate(EstimateInput{Provider: "aws", InstanceType: "t3.micro", Count: 1})
	require.NoError(t, err)
	assert.Equal(t, 1.0, est.Hours)
	assert.Equal(t, 0.0104, est.TotalUSD)
}

func TestComputeEstimate_UnknownRateZero(t *testing.T) {
	est, err := ComputeEstimate(EstimateInput{Provider: "hetzner", InstanceType: "nope", Count: 1, Hours: 2})
	require.NoError(t, err)
	assert.False(t, est.KnownRate)
	assert.Equal(t, 0.0, est.TotalUSD)
}

func TestComputeEstimate_Invalid(t *testing.T) {
	_, err := ComputeEstimate(EstimateInput{Provider: "", InstanceType: "x"})
	assert.Error(t, err)
	_, err = ComputeEstimate(EstimateInput{Provider: "hetzner", InstanceType: "cx21", Count: 0})
	assert.Error(t, err)
}

func TestIdleEvaluator_AllSamplesLow(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1, CPUThreshold: 5}, func(string, time.Time) ([]float64, bool) {
		return []float64{1, 2, 3, 4}, true
	})
	st := eval.Evaluate("a1", time.Now())
	assert.True(t, st.Idle)
	assert.False(t, st.Actionable) // AutoDestroy default false
	assert.Equal(t, 1.0, st.IdleHours)
}

func TestIdleEvaluator_AnySampleHigh(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1, CPUThreshold: 5}, func(string, time.Time) ([]float64, bool) {
		return []float64{1, 8, 3}, true
	})
	st := eval.Evaluate("a1", time.Now())
	assert.False(t, st.Idle)
	assert.Contains(t, st.Reason, "above")
}

func TestIdleEvaluator_NoDataUnknown(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{IdleHours: 1, CPUThreshold: 5}, func(string, time.Time) ([]float64, bool) {
		return nil, false
	})
	st := eval.Evaluate("a1", time.Now())
	assert.False(t, st.Idle)
	assert.Contains(t, st.Reason, "no recent")
}

func TestIdleEvaluator_AutoDestroyActionable(t *testing.T) {
	eval := NewIdleEvaluator(IdlePolicy{Enabled: true, AutoDestroy: true, IdleHours: 2, CPUThreshold: 3}, func(string, time.Time) ([]float64, bool) {
		return []float64{1, 1}, true
	})
	st := eval.Evaluate("a1", time.Now())
	assert.True(t, st.Idle)
	assert.True(t, st.Actionable)
}

func TestBuildSummary(t *testing.T) {
	now := time.Now()
	runs := []model.ProvisionRun{
		{RunID: "r1", Provider: "hetzner", GameID: "g", Env: "e", Region: "eu", State: "adopted",
			CreatedAt:    now.Add(-2 * time.Hour),
			SpecJSON:     model.JSON(`{"instanceType":"cx21"}`),
			AgentIDsJSON: model.JSON(`["a1"]`)},
		{RunID: "r2", Provider: "aws", GameID: "g", Env: "e", Region: "us", State: "adopted",
			CreatedAt:    now.Add(-1 * time.Hour),
			SpecJSON:     model.JSON(`{"instanceType":"t3.micro"}`),
			AgentIDsJSON: model.JSON(`["a2"]`)},
	}
	sum := BuildSummary(runs, map[string]bool{"a1": true}, map[string]float64{"a1": 1.5}, now)
	assert.Equal(t, 2, sum.TotalNodes)
	assert.Equal(t, 1, sum.IdleCount)
	// a1 idle (2h * 0.006) + a2 (1h * 0.0104)
	assert.InDelta(t, 2*0.006+1*0.0104, sum.TotalUSD, 1e-9)
	assert.Equal(t, "a1", sum.Nodes[0].AgentID) // cost desc: a1 0.012 > a2 0.0104
}

func TestBudgetEvaluator_NoConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	eval := &BudgetEvaluator{Store: model.NewCostBudgetModel(db)}
	alert, err := eval.Evaluate(context.Background(), "g", "e", 10)
	require.NoError(t, err)
	assert.Nil(t, alert)
}

// 审核发现：Evaluate 把 Store.Get 的任意错误当「未配置预算」返回 nil——真实
// DB 故障时预算告警静默失效。回归：非 RecordNotFound 错误必须上抛。
func TestBudgetEvaluator_DBErrorPropagates(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	// 关闭底层 DB，令 Get 返回非 RecordNotFound 的真实错误。
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	eval := &BudgetEvaluator{Store: model.NewCostBudgetModel(db)}
	alert, err := eval.Evaluate(context.Background(), "g", "e", 10)
	require.Error(t, err, "真实 DB 故障不得被当作未配置预算")
	assert.Nil(t, alert)
}

func TestBudgetEvaluator_UnderThreshold(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	m := model.NewCostBudgetModel(db)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e", MonthlyUSD: 100}))
	eval := &BudgetEvaluator{Store: m, ThresholdPct: 80}
	alert, err := eval.Evaluate(context.Background(), "g", "e", 50)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.False(t, alert.OverThreshold)
}

func TestBudgetEvaluator_OverThreshold(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	m := model.NewCostBudgetModel(db)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e", MonthlyUSD: 100}))
	eval := &BudgetEvaluator{Store: m, ThresholdPct: 80}
	alert, err := eval.Evaluate(context.Background(), "g", "e", 90)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.True(t, alert.OverThreshold)
}

func TestAggregateByScope(t *testing.T) {
	now := time.Now()
	runs := []model.ProvisionRun{
		{RunID: "r1", Provider: "hetzner", GameID: "g", Env: "e", CreatedAt: now.Add(-2 * time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"cx21"}`)},
		{RunID: "r2", Provider: "aws", GameID: "g2", Env: "e", CreatedAt: now.Add(-1 * time.Hour),
			SpecJSON: model.JSON(`{"instanceType":"t3.micro"}`)},
	}
	out := AggregateByScope(runs, now)
	assert.InDelta(t, 2*0.006, out["g/e"], 1e-9)
	assert.InDelta(t, 1*0.0104, out["g2/e"], 1e-9)
}

func TestRateAndProviders(t *testing.T) {
	_, ok := Rate("hetzner", "cx21")
	assert.True(t, ok)
	_, ok = Rate("hetzner", "nope")
	assert.False(t, ok)
	assert.Equal(t, "aws", NormalizeProvider(" AWS "))
	assert.Contains(t, FormatProviders(), "hetzner")
}

func TestIdlePolicyValidate(t *testing.T) {
	assert.NoError(t, IdlePolicy{IdleHours: 24, CPUThreshold: 5}.Validate())
	assert.Error(t, IdlePolicy{IdleHours: 0, CPUThreshold: 5}.Validate())
	assert.Error(t, IdlePolicy{IdleHours: 24, CPUThreshold: 101}.Validate())
}

func TestCostBudgetModel(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}))
	m := model.NewCostBudgetModel(db)
	require.NoError(t, m.Upsert(context.Background(), &model.CostBudget{GameID: "g", Env: "e", MonthlyUSD: 50, NotifyChannelsJSON: `["ding_main"]`}))
	b, err := m.Get(context.Background(), "g", "e")
	require.NoError(t, err)
	assert.Equal(t, 50.0, b.MonthlyUSD)
	var ch []string
	require.NoError(t, json.Unmarshal([]byte(b.NotifyChannelsJSON), &ch))
	assert.Equal(t, []string{"ding_main"}, ch)
}
