package ops

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/costcontrol"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func costTestSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CostBudget{}, &model.ProvisionRun{}))
	metrics := registry.NewMetricsStore()
	return &svc.ServiceContext{DB: db, MetricsStore: metrics}
}

func TestCostEstimate_Service(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	est, err := s.CostEstimate(context.Background(), &CostEstimateRequest{Provider: "hetzner", InstanceType: "cx21", Count: 2, Hours: 10})
	require.NoError(t, err)
	assert.InDelta(t, 0.12, est.TotalUSD, 1e-9)
}

func TestCostEstimate_Invalid(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	_, err := s.CostEstimate(context.Background(), &CostEstimateRequest{Provider: "", InstanceType: ""})
	assert.Error(t, err)
	_, err = s.CostEstimate(context.Background(), nil)
	assert.Error(t, err)
}

func TestCostBudgetUpsertGet(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	b, err := s.CostBudgetUpsert(context.Background(), "g", "e", &CostBudgetUpsertRequest{MonthlyUSD: 500, NotifyChannels: []string{"ding"}}, "admin")
	require.NoError(t, err)
	assert.Equal(t, 500.0, b.MonthlyUSD)

	got, err := s.CostBudgetGet(context.Background(), "g", "e")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 500.0, got.MonthlyUSD)
	assert.Equal(t, []string{"ding"}, got.NotifyChannels)
}

func TestCostBudgetGet_Missing(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	got, err := s.CostBudgetGet(context.Background(), "g", "e")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// 审核发现：旧实现以 strings.Contains(err.Error(), "not found") 判定未配置，
// 任何消息含 "not found" 的 DB 错误（如表/库不存在）都会被误判为无预算。
// 回归：真实 DB 故障必须上抛，只有 RecordNotFound 返回 nil。
func TestCostBudgetGet_DBErrorPropagates(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	sqlDB, err := s.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	got, err := s.CostBudgetGet(context.Background(), "g", "e")
	require.Error(t, err, "真实 DB 故障不得被误判为未配置预算")
	assert.Nil(t, got)
}

func TestCostSummary_ScopedAndIdle(t *testing.T) {
	ctx := context.Background()
	s := &Service{svcCtx: costTestSvcCtx(t)}
	db := s.svcCtx.DB
	runModel := model.NewProvisionRunModel(db)
	now := time.Now()
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-x", Provider: "hetzner", Region: "eu", GameID: "g", Env: "e",
		State: "adopted", Count: 1, CreatedAt: now.Add(-2 * time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"cx21"}`), AgentIDsJSON: model.JSON(`["a1"]`),
	}))
	// 另一 scope 的运行不计入。
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-y", Provider: "aws", Region: "us", GameID: "other", Env: "e",
		State: "adopted", Count: 1, CreatedAt: now.Add(-1 * time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"t3.micro"}`), AgentIDsJSON: model.JSON(`["a2"]`),
	}))
	// 非 adopted 不计入。
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-z", Provider: "aws", Region: "us", GameID: "g", Env: "e",
		State: "failed", Count: 1, CreatedAt: now.Add(-1 * time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"t3.micro"}`), AgentIDsJSON: model.JSON(`["a3"]`),
	}))
	sum, err := s.CostSummary(ctx, "g", "e", 100)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.TotalNodes)
	assert.Equal(t, 0, sum.IdleCount) // 无指标样本 → 未知，不误判
	assert.InDelta(t, 2*0.006, sum.TotalUSD, 1e-7)
}

func TestCostSummary_NoDB(t *testing.T) {
	s := &Service{svcCtx: &svc.ServiceContext{}}
	_, err := s.CostSummary(context.Background(), "g", "e", 10)
	assert.Error(t, err)
}

// 审核发现：旧实现先全局 limit 截断再内存过滤，目标 scope 的较早 adopted
// 运行若落在最新 200 条之外会被静默漏计。回归：scope/state 过滤下推 SQL，
// 目标 scope 的旧运行仍被统计。
func TestCostSummary_ScopeFilterPushedDown(t *testing.T) {
	ctx := context.Background()
	s := &Service{svcCtx: costTestSvcCtx(t)}
	db := s.svcCtx.DB
	runModel := model.NewProvisionRunModel(db)
	now := time.Now()

	// 先造一个其它 scope 的较新 adopted 运行（占掉全局 limit 前排）。
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-new", Provider: "aws", Region: "us", GameID: "other", Env: "e",
		State: "adopted", Count: 1, CreatedAt: now.Add(-1 * time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"t3.micro"}`), AgentIDsJSON: model.JSON(`["a2"]`),
	}))
	// 目标 scope 的较旧 adopted 运行（时间在其它 scope 之前，但仍在目标内）。
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-old", Provider: "hetzner", Region: "eu", GameID: "g", Env: "e",
		State: "adopted", Count: 1, CreatedAt: now.Add(-48 * time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"cx21"}`), AgentIDsJSON: model.JSON(`["a1"]`),
	}))

	// limit=1：旧实现全局取最新 1 条（other scope），过滤后空 → TotalNodes=0；
	// 修复后 scope 下推仍取到目标 scope 的旧运行。
	sum, err := s.CostSummary(ctx, "g", "e", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.TotalNodes, "目标 scope 的旧运行不得因全局 limit 被漏计")
	assert.InDelta(t, 48*0.006, sum.TotalUSD, 1e-6)
}

var _ = costcontrol.DefaultIdlePolicy
