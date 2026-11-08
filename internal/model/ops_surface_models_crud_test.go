package model

// ops 面（incident/db_source/deployment/runbook/provision/task/external
// token/probe event）模型层 CRUD 补测：这些方法此前无任何用例触达。
// 统一 sqlite 内存库 + AutoMigrate；:memory: 每池连接是独立空库，钉单连接。

import (
	"context"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOpsSurfaceDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

func TestExternalTokenModelCRUD(t *testing.T) {
	db := newOpsSurfaceDB(t, &ExternalToken{})
	m := NewExternalTokenModel(db)
	ctx := context.Background()

	row := &ExternalToken{Name: "ci", TokenHash: "hash-1", Scope: []byte(`{"categories":["game","ops"]}`)}
	require.NoError(t, m.Create(ctx, row))
	require.NotZero(t, row.ID)

	got, err := m.FindByHash(ctx, "hash-1")
	require.NoError(t, err)
	assert.Equal(t, "ci", got.Name)

	_, err = m.FindByHash(ctx, "hash-missing")
	assert.Error(t, err)

	rows, total, err := m.List(ctx, ListTokensOptions{Page: 0, PageSize: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)

	assert.Equal(t, []string{"game", "ops"}, row.ScopeCategories())
	assert.Nil(t, (&ExternalToken{}).ScopeCategories())
	assert.Nil(t, (&ExternalToken{Scope: []byte(`not json`)}).ScopeCategories())

	require.NoError(t, m.Update(ctx, row.ID, map[string]interface{}{"name": "ci-2"}))
	require.NoError(t, m.TouchUsed(ctx, row.ID, time.Now()))
	require.NoError(t, m.Delete(ctx, row.ID))
	_, err = m.FindByHash(ctx, "hash-1")
	assert.Error(t, err)
}

func TestIncidentModelQuerySurface(t *testing.T) {
	db := newOpsSurfaceDB(t, &Incident{}, &IncidentCategory{})
	ctx := context.Background()
	im := NewIncidentModel(db)

	cat := &IncidentCategory{Name: "db", Slug: "db", Enabled: true, Builtin: false}
	require.NoError(t, db.Create(cat).Error)
	uncat := &IncidentCategory{Name: "未分类", Slug: IncidentCatUncategorized, Enabled: true, Builtin: true}
	require.NoError(t, db.Create(uncat).Error)

	key := "inc-1"
	inc := &Incident{IncidentKey: &key, Title: "t", CategoryID: cat.ID, Severity: "high", Status: "open", Source: "alert", GameID: "g", Env: "e", DetectedAt: time.Now()}
	require.NoError(t, db.Create(inc).Error)

	got, err := im.FindByKey(ctx, "inc-1")
	require.NoError(t, err)
	assert.Equal(t, "t", got.Title)
	_, err = im.FindByKey(ctx, "inc-missing")
	assert.Error(t, err)

	require.NoError(t, im.Update(ctx, inc.ID, map[string]interface{}{"status": "resolved"}))

	rows, total, err := im.List(ctx, IncidentQueryOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		CategoryID:        cat.ID, Status: "resolved", Severity: "high", Source: "alert",
		GameID: "g", Env: "e",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)

	now := time.Now()
	rows, total, err = im.List(ctx, IncidentQueryOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		From:              &now,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)

	require.NoError(t, im.ReassignCategory(ctx, cat.ID, uncat.ID))
	n, err := im.CountByCategory(ctx, uncat.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestIncidentCategoryModelGuards(t *testing.T) {
	db := newOpsSurfaceDB(t, &IncidentCategory{})
	m := NewIncidentCategoryModel(db)
	ctx := context.Background()

	on := &IncidentCategory{Name: "a", Slug: "a", Enabled: true}
	off := &IncidentCategory{Name: "b", Slug: "b", Enabled: false}
	require.NoError(t, db.Create(on).Error)
	require.NoError(t, db.Create(off).Error)

	all, err := m.List(ctx, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	enabled, err := m.List(ctx, true)
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, "a", enabled[0].Name)

	require.NoError(t, m.Update(ctx, on.ID, map[string]interface{}{"leader": "op"}))
	err = m.Update(ctx, 9999, map[string]interface{}{"leader": "op"})
	assert.Error(t, err, "missing row must surface record-not-found")

	missing, err := m.MissingEnabledIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, missing)
	missing, err = m.MissingEnabledIDs(ctx, []uint{on.ID, off.ID, 9999})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint{off.ID, 9999}, missing)

	require.NoError(t, m.Delete(ctx, off.ID))
	id, err := m.UncategorizedID(ctx)
	require.Error(t, err, "uncategorized row missing must error")
	assert.Zero(t, id)
}

func TestDBSourceOwnerScopedQueries(t *testing.T) {
	db := newOpsSurfaceDB(t, &DBSource{})
	m := NewDBSourceModel(db)
	ctx := context.Background()

	rows := []*DBSource{
		{Name: "a", OwnerAgentID: "ag-1", GameID: "g", Env: "prod"},
		{Name: "b", OwnerAgentID: "ag-1", GameID: "g", Env: "prod"},
		{Name: "c", OwnerAgentID: "ag-1", GameID: "g", Env: "prod"},
		{Name: "d", OwnerAgentID: "ag-2", GameID: "g", Env: "prod"},
		{Name: "e", OwnerAgentID: "ag-1", GameID: "g2", Env: "dev"},
	}
	for _, r := range rows {
		require.NoError(t, db.Create(r).Error)
	}
	// Enabled 带 default:true tag，Create 时零值会被默认值顶掉，
	// 停用必须走显式 Update。
	require.NoError(t, m.Update(ctx, rows[2].ID, map[string]interface{}{"enabled": false}))

	mine, err := m.ListByOwner(ctx, "ag-1")
	require.NoError(t, err)
	require.Len(t, mine, 3, "all enabled sources of ag-1 across scopes, id ASC")
	assert.Equal(t, "a", mine[0].Name)

	scoped, err := m.ListByOwnerAndScope(ctx, "ag-1", "g", "prod")
	require.NoError(t, err)
	require.Len(t, scoped, 2)

	src, err := m.FindByScope(ctx, "g", "prod")
	require.NoError(t, err)
	assert.Equal(t, "a", src.Name)
	_, err = m.FindByScope(ctx, "g", "stage")
	assert.Error(t, err)
}

func TestDeploymentAndPackageLists(t *testing.T) {
	db := newOpsSurfaceDB(t, &Deployment{}, &DeployPackage{})
	dm := NewDeploymentModel(db)
	pm := NewDeployPackageModel(db)
	ctx := context.Background()

	for _, id := range []string{"d1", "d2", "d3"} {
		require.NoError(t, dm.Create(ctx, &Deployment{DeploymentID: id, PackageID: "p", GameID: "g", Env: "e"}))
	}
	ds, err := dm.List(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, ds, 2)
	ds, err = dm.List(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, ds, 3)

	require.NoError(t, pm.Create(ctx, &DeployPackage{PackageID: "pkg-1"}))
	got, err := pm.FindByID(ctx, "pkg-1")
	require.NoError(t, err)
	assert.Equal(t, "pkg-1", got.PackageID)
	_, err = pm.FindByID(ctx, "pkg-missing")
	assert.Error(t, err)
	pkgs, err := pm.List(ctx, 5)
	require.NoError(t, err)
	assert.Len(t, pkgs, 1)
}

func TestRunbookRunRecoveryScan(t *testing.T) {
	db := newOpsSurfaceDB(t, &RunbookRun{})
	m := NewRunbookRunModel(db)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)

	require.NoError(t, db.Create(&RunbookRun{RunID: "r1", RunbookID: "rb", GameID: "g", Env: "e", Status: RunbookRunStatusRunning, StartedAt: old}).Error)
	require.NoError(t, db.Create(&RunbookRun{RunID: "r2", RunbookID: "rb", GameID: "g", Env: "e", Status: RunbookRunStatusRunning, StartedAt: time.Now()}).Error)
	fin := time.Now()
	require.NoError(t, db.Create(&RunbookRun{RunID: "r3", RunbookID: "rb", GameID: "g", Env: "e", Status: "succeeded", StartedAt: old, FinishedAt: &fin}).Error)

	stale, err := m.ListRunning(ctx, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, "r1", stale[0].RunID)
}

func TestProvisionRunScopeList(t *testing.T) {
	db := newOpsSurfaceDB(t, &ProvisionRun{})
	m := NewProvisionRunModel(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&ProvisionRun{RunID: "p1", Provider: "test", GameID: "g", Env: "prod", State: "adopted"}).Error)
	require.NoError(t, db.Create(&ProvisionRun{RunID: "p2", Provider: "test", GameID: "g2", Env: "dev", State: "adopted"}).Error)
	require.NoError(t, db.Create(&ProvisionRun{RunID: "p3", Provider: "test", GameID: "g", Env: "prod", State: "failed"}).Error)

	all, err := m.List(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	scoped, err := m.ListAdoptedScope(ctx, "g", "prod", 0)
	require.NoError(t, err)
	require.Len(t, scoped, 1)
	assert.Equal(t, "p1", scoped[0].RunID)

	scoped, err = m.ListAdoptedScope(ctx, "", "", 1)
	require.NoError(t, err)
	assert.Len(t, scoped, 1)
}

func TestProbeEventModelCreateAndRecent(t *testing.T) {
	db := newOpsSurfaceDB(t, &ProbeEvent{})
	m := NewProbeEventModel(db)
	ctx := context.Background()

	var nilModel *ProbeEventModel
	_, err := nilModel.Recent(ctx, "g", "e", "", 10)
	assert.Error(t, err)
	assert.ErrorIs(t, nilModel.Create(ctx, "a", "g", "e", &opsv1.AlertEvent{}), gorm.ErrInvalidDB)

	window := &opsv1.AlertEvent{Kind: "probe.window", Window: &opsv1.ProbeWindow{
		Target: "mysql://g", WindowKind: "unavailable", LastReason: "dial timeout", TargetCategory: "game",
	}}
	require.NoError(t, m.Create(ctx, "a1", "g", "prod", window))

	detail := &opsv1.AlertEvent{Kind: "probe.detail", Detail: &opsv1.ProbeDetail{
		Target: "redis://g", Ok: false, Err: "EOF", LatencyMs: 12,
	}}
	require.NoError(t, m.Create(ctx, "a1", "g", "prod", detail))
	require.NoError(t, m.Create(ctx, "a2", "g", "prod", &opsv1.AlertEvent{Kind: "probe.detail"}))

	rows, err := m.Recent(ctx, "g", "prod", "", 10)
	require.NoError(t, err)
	require.Len(t, rows, 3)

	byTarget := func(target string) ProbeEvent {
		for _, r := range rows {
			if r.Target == target {
				return r
			}
		}
		return ProbeEvent{}
	}
	win := byTarget("mysql://g")
	assert.Equal(t, "unavailable", win.Status)
	assert.Equal(t, "timeout", win.ErrorClass)
	assert.Equal(t, "game", win.TargetCategory)
	det := byTarget("redis://g")
	assert.Equal(t, "failed", det.Status)
	assert.Equal(t, int64(12), det.LatencyMS)

	rows, err = m.Recent(ctx, "g", "prod", "a1", 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	rows, err = m.Recent(ctx, "g", "prod", "", 0)
	require.NoError(t, err)
	require.Len(t, rows, 3, "limit below 1 clamps to default 100")
}

func TestTaskRunReapByAgent(t *testing.T) {
	db := newOpsSurfaceDB(t, &TaskRun{})
	m := NewTaskRunModel(db)
	ctx := context.Background()

	_, err := m.ReapByAgent(ctx, "  ", []string{"running"}, "failed", "agent gone")
	assert.Error(t, err, "blank agent id must be rejected")

	n, err := m.ReapByAgent(ctx, "ag-1", nil, "failed", "agent gone")
	require.NoError(t, err)
	assert.Zero(t, n)

	now := time.Now()
	require.NoError(t, db.Create(&TaskRun{TaskID: "t1", AgentID: "ag-1", Status: "running", StartedAt: &now}).Error)
	require.NoError(t, db.Create(&TaskRun{TaskID: "t2", AgentID: "ag-1", Status: "succeeded", StartedAt: &now}).Error)
	require.NoError(t, db.Create(&TaskRun{TaskID: "t3", AgentID: "ag-2", Status: "running", StartedAt: &now}).Error)

	n, err = m.ReapByAgent(ctx, "ag-1", []string{"running"}, "failed", "agent gone")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	var reaped TaskRun
	require.NoError(t, db.Where("task_id = ?", "t1").First(&reaped).Error)
	assert.Equal(t, "failed", reaped.Status)
	assert.Equal(t, "agent gone", reaped.Message)
	assert.NotNil(t, reaped.FinishedAt)

	var untouched TaskRun
	require.NoError(t, db.Where("task_id = ?", "t3").First(&untouched).Error)
	assert.Equal(t, "running", untouched.Status)
}
