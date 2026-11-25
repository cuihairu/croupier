package incident

// Extra coverage for service.go (failpoint injection style): every "model-layer
// error passthrough" arm. The DB failure arms of List/Get/Create/Update/Count
// inject deterministic errors via GORM callbacks (After injection for
// Query/Update/Create/Delete, Before injection for the Scan row processor to
// avoid connection leaks, all targeted precisely by table plus hit count); the
// idempotency-key unique-conflict arm is triggered for real via a "soft-deleted
// row squatting the slot" trick (the unique index does not filter deleted_at, so
// Find misses under gorm soft-delete filtering while Create hits the real
// constraint).
//
// Honest skips: the FindByKey hit arm of CreateIncident's idempotency conflict
// recovery (service.go 226-228) - that same query has to miss first and hit
// later, which only a concurrent race window can produce, so it cannot be
// covered reliably; DispatchLifecycle's ev==nil defensive arm (webhook.go
// 38-39) - lifecycleEvent always builds an event from a non-nil row, so it is
// unreachable.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// dbFailpoint describes one deterministic failure injection: err is injected on
// the nth time the op operation reaches table (the callback stage sets
// tx.AddError, so the caller returns with that error).
//
// Scan (row processor) must be injected with Before: gorm's Scan does not Close
// an already-opened *sql.Rows when Rows() returns an error (connection leak).
// Before injection makes RowQuery skip execution entirely and never touch the
// connection pool; After injection leaks the connection (under an unbounded pool
// this surfaces as later queries hitting an empty :memory: database on a fresh
// connection, and under a single-connection pool it deadlocks).
type dbFailpoint struct {
	table string // incidents / bugs / incident_categories / incident_reports / external_tokens
	op    string // query / update / create / delete
	n     int    // inject on the nth hit (1-based)
	err   error
}

// applyDBFailpoint registers the injection callback on the test DB (each test
// has its own DB, so the callbacks never interfere).
func applyDBFailpoint(t *testing.T, db *gorm.DB, fp dbFailpoint) {
	t.Helper()
	var hits atomic.Int64
	reg := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != fp.table {
			return
		}
		if hits.Add(1) == int64(fp.n) {
			_ = tx.AddError(fp.err)
		}
	}
	name := "test:fail_" + fp.table + "_" + fp.op
	switch fp.op {
	case "query":
		// Scan (Rows) goes through the row processor (Before injection prevents
		// connection leaks); Find/First/Count go through the Query processor
		// (After injection is safe: rows are closed right after being read).
		// Both sides share the same counter, forming one unified query order for
		// the table.
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name+"_row", reg))
	case "update":
		require.NoError(t, db.Callback().Update().After("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().After("gorm:create").Register(name, reg))
	case "delete":
		require.NoError(t, db.Callback().Delete().After("gorm:delete").Register(name, reg))
	default:
		t.Fatalf("unknown failpoint op %q", fp.op)
	}
}

var errFailpoint = errors.New("failpoint: storage down")

func TestListCategories_ListError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err := s.ListCategories(context.Background(), false)
	require.ErrorIs(t, err, errFailpoint)
}

func TestCreateCategory_CreateError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "create", n: 1, err: errFailpoint})
	_, err := s.CreateCategory(context.Background(), &CategoryUpsertRequest{Name: strPtr("新域"), Slug: "new"})
	require.ErrorIs(t, err, errFailpoint)
}

func TestUpdateCategory_UpdateErrorAndReGetError(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	// The Update itself fails (not a unique conflict -> passed through as-is)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "update", n: 1, err: errFailpoint})
	_, err := s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Leader: strPtr("op")})
	require.ErrorIs(t, err, errFailpoint)

	// The read-back after a successful Update fails
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Leader: strPtr("op2")})
	require.ErrorIs(t, err, errFailpoint)
}

func TestCategoryUsage_CountErrors(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	// The incidents count fails
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.CategoryUsage(ctx, qa.ID)
	require.ErrorIs(t, err, errFailpoint)

	// The bugs count fails
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err = s.CategoryUsage(ctx, qa.ID)
	require.ErrorIs(t, err, errFailpoint)
}

func TestDeleteCategory_ErrorArms(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	tmp, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("临时域"), Slug: "tmp"})
	require.NoError(t, err)

	// The unclassified fallback row lookup fails (2nd incident_categories query)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	require.ErrorIs(t, s.DeleteCategory(ctx, tmp.ID), errFailpoint)

	// The in-transaction incidents merge fails -> rollback
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "update", n: 1, err: errFailpoint})
	require.ErrorIs(t, s.DeleteCategory(ctx, tmp.ID), errFailpoint)

	// The in-transaction bugs merge fails -> rollback
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "update", n: 1, err: errFailpoint})
	require.ErrorIs(t, s.DeleteCategory(ctx, tmp.ID), errFailpoint)
}

func TestCreateIncident_FindByKeyError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	qa := mustCategory(t, s, model.IncidentCatQA)
	_, _, err := s.CreateIncident(context.Background(),
		&IncidentCreateRequest{Title: "x", CategoryID: qa.ID, IncidentKey: "k1"}, "tester")
	require.ErrorIs(t, err, errFailpoint)
}

// A soft-deleted row squats the slot: FindByKey (soft-delete filtered) misses
// -> Create hits the real unique index -> isUniqueViolation matches -> the
// conflict recovery query still misses -> explicit conflict.
func TestCreateIncident_IdempotencyConflictWithSoftDeletedRow(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	key := "ext:dup-1"
	seed := &model.Incident{Title: "soft-deleted", CategoryID: qa.ID, IncidentKey: &key,
		Severity: model.IncidentSeverityInfo, Source: model.IncidentSourceExternal,
		DetectedAt: time.Now()}
	seed.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	require.NoError(t, s.svcCtx.DB.Create(seed).Error)

	dto, created, err := s.CreateIncident(ctx,
		&IncidentCreateRequest{Title: "replay", CategoryID: qa.ID, IncidentKey: key}, "tester")
	require.ErrorContains(t, err, "事故幂等键已存在")
	require.Nil(t, dto)
	require.False(t, created)
}

func TestCreateIncident_CreateError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "create", n: 1, err: errFailpoint})
	qa := mustCategory(t, s, model.IncidentCatQA)
	_, _, err := s.CreateIncident(context.Background(),
		&IncidentCreateRequest{Title: "x", CategoryID: qa.ID}, "tester")
	require.ErrorIs(t, err, errFailpoint)
}

func TestGetIncident_GetError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.GetIncident(context.Background(), 1)
	require.ErrorIs(t, err, errFailpoint)
}

// Contract after the AUD-020 fix: a Details update is JSON encoded, lands in the
// database, and reads back correctly. This path used to bypass the JSON
// serializer on map updates and report "unsupported type
// map[string]interface {}"; this test once pinned that buggy behavior, and now
// asserts a successful write after the fix.
func TestUpdateIncident_DetailsJSONPersisted(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "d", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	upd, err := s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{
		Details: map[string]interface{}{"zone": "cn"},
	})
	require.NoError(t, err)
	require.Equal(t, "cn", upd.Details["zone"])
}

func TestUpdateIncident_UpdateErrorAndReGetError(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "u", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	// The Update itself fails
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "update", n: 1, err: errFailpoint})
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{GameID: strPtr("g1")})
	require.ErrorIs(t, err, errFailpoint)

	// The read-back after a successful Update fails (Get's first read is hit #1,
	// the read-back is #2)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 2, err: errFailpoint})
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{GameID: strPtr("g2")})
	require.ErrorIs(t, err, errFailpoint)
}

func TestTransitionIncident_UpdateErrorAndReGetError(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "t", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "update", n: 1, err: errFailpoint})
	_, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: model.IncidentStatusAcked})
	require.ErrorIs(t, err, errFailpoint)

	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 2, err: errFailpoint})
	_, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: model.IncidentStatusOpen})
	require.ErrorIs(t, err, errFailpoint)
}

func TestCreateIncident_CategoryGetError(t *testing.T) {
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, _, err := s.CreateIncident(context.Background(),
		&IncidentCreateRequest{Title: "x", CategoryID: 1}, "tester")
	require.ErrorIs(t, err, errFailpoint)
}

func TestCreateCategory_AppliesEnabledFlag(t *testing.T) {
	s := setupService(t)
	dto, err := s.CreateCategory(context.Background(),
		&CategoryUpsertRequest{Name: strPtr("停用域"), Slug: "pre-disabled", Enabled: boolPtr(false)})
	require.NoError(t, err)
	require.False(t, dto.Enabled)
}

func TestListIncidents_CategoryNamesErrorSwallowed(t *testing.T) {
	s := setupService(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	_, _, err := s.CreateIncident(context.Background(),
		&IncidentCreateRequest{Title: "n", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	resp, err := s.ListIncidents(context.Background(), model.IncidentQueryOptions{})
	require.NoError(t, err, "类别名回查失败按空映射降级")
	require.Len(t, resp.Items, 1)
	require.Empty(t, resp.Items[0].CategorySlug, "降级后无类别冗余字段")
}

// ---- external.go error / guard arms ----

func TestNewExternalService_DefaultAndCustomRPM(t *testing.T) {
	db := setupService(t).svcCtx.DB

	def := NewExternalService(svcCtxOf(t, db, 0))
	require.NotNil(t, def)
	require.NotNil(t, def.limiter)
	t.Cleanup(func() { _ = def.limiter.Close() })

	custom := NewExternalService(svcCtxOf(t, db, 50))
	require.NotNil(t, custom)
	t.Cleanup(func() { _ = custom.limiter.Close() })
}

func TestResolveToken_FindError(t *testing.T) {
	s, db := setupExternal(t)
	applyDBFailpoint(t, db, dbFailpoint{table: "external_tokens", op: "query", n: 1, err: errFailpoint})
	_, err := s.ResolveToken(context.Background(), "plain")
	require.ErrorIs(t, err, errFailpoint)
}

func TestTouchUsed_NilToken(t *testing.T) {
	s, _ := setupExternal(t)
	s.TouchUsed(context.Background(), nil)
}

func TestCheckRateLimit_ClosedLimiter(t *testing.T) {
	s, _ := setupExternal(t)
	require.NoError(t, s.limiter.Close())
	err := s.CheckRateLimit(context.Background(), "probe")
	require.ErrorContains(t, err, "限流失效", "限流器失效错误须包装为内部错误")
}

func TestExternalRegisterIncident_ExplicitSourceWins(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	qa := mustCategory(t, s.incSvc, model.IncidentCatQA)
	tok := &model.ExternalToken{Name: "probe"}

	dto, created, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "ext", CategoryID: qa.ID, Source: model.IncidentSourceAlert,
	}, tok, "ext-key-1")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, model.IncidentSourceAlert, dto.Source, "显式来源须覆盖 external 默认")
}

func TestExternalListIncidents_ListError(t *testing.T) {
	s, db := setupExternal(t)
	applyDBFailpoint(t, db, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.ListIncidents(context.Background(), model.IncidentQueryOptions{}, nil)
	require.ErrorIs(t, err, errFailpoint)
}

func TestExternalCreateBug_ValidationError(t *testing.T) {
	s, _ := setupExternal(t)
	_, err := s.CreateBug(context.Background(), &bug.BugCreateRequest{}, &model.ExternalToken{Name: "probe"})
	require.ErrorContains(t, err, "缺陷标题不能为空")
}

func TestExternalScopeMap_NilToken(t *testing.T) {
	s, _ := setupExternal(t)
	require.Empty(t, s.scopeMap(nil))
}

func TestSendLifecycleWebhook_Dispatches(t *testing.T) {
	s, _ := setupExternal(t)
	s.SendLifecycleWebhook(context.Background(), &model.Incident{Title: "w"}, LifecycleIncidentCreated)
}

// ---- helpers ----

func svcCtxOf(t *testing.T, db *gorm.DB, rpm int) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{DB: db, Config: config.Config{
		ExternalAPI: config.ExternalAPIConfig{RateLimitPerMinute: rpm},
	}}
}
