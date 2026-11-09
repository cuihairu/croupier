package incident

// service.go 补测（failpoint 注入口径）：全部「模型层错误透传」臂——
// List/Get/Create/Update/Count 的 DB 失败臂经 GORM 回调注入确定性错误
// （Query/Update/Create/Delete 用 After 注入、Scan 的 row 处理器用
// Before 注入防连接泄漏，均按表+次数精确定位目标查询）；幂等键唯一
// 冲突臂用「软删行占坑」真实触发（唯一索引不含 deleted_at 过滤，Find
// 按 gorm 软删过滤 miss，Create 撞真约束）。
//
// 诚实跳过：CreateIncident 幂等冲突恢复的 FindByKey 命中臂（service.go
// 226-228）——同一查询需先 miss 后 hit，只能并发竞窗触发，不可稳定覆盖；
// DispatchLifecycle 的 ev==nil 防御臂（webhook.go 38-39）——lifecycleEvent
// 由非 nil row 恒构造出事件，不可达。

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

// dbFailpoint 描述一次确定性失败注入：第 n 次触达 table 的 op 操作时注入
// err（回调阶段置 tx.AddError，调用方按错误返回）。
//
// Scan（row 处理器）必须用 Before 注入：gorm 的 Scan 在 Rows() 返回错误时
// 不会 Close 已打开的 *sql.Rows（连接泄漏）——Before 注入令 RowQuery 直接
// 跳过执行，不触碰连接池；After 注入会泄漏连接（无界池下表现为后续查询
// 撞新连接的空 :memory: 库，单连接池下死锁）。
type dbFailpoint struct {
	table string // incidents / bugs / incident_categories / incident_reports / external_tokens
	op    string // query / update / create / delete
	n     int    // 第 n 次命中注入（1 起）
	err   error
}

// applyDBFailpoint 在测试 DB 上注册注入回调（每个测试独享 DB，回调互不干扰）。
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
		// Scan（Rows）走 row 处理器（Before 注入防连接泄漏），Find/First/Count
		// 走 Query 处理器（After 注入安全：行读毕即 Close）——两侧共用同一计数，
		// 形成该表的统一查询序。
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

	// Update 本体失败（非唯一冲突 → 原样透传）
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "update", n: 1, err: errFailpoint})
	_, err := s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Leader: strPtr("op")})
	require.ErrorIs(t, err, errFailpoint)

	// Update 成功后回读失败
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Leader: strPtr("op2")})
	require.ErrorIs(t, err, errFailpoint)
}

func TestCategoryUsage_CountErrors(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	// incidents 计数失败
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.CategoryUsage(ctx, qa.ID)
	require.ErrorIs(t, err, errFailpoint)

	// bugs 计数失败
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err = s.CategoryUsage(ctx, qa.ID)
	require.ErrorIs(t, err, errFailpoint)
}

func TestDeleteCategory_ErrorArms(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	tmp, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("临时域"), Slug: "tmp"})
	require.NoError(t, err)

	// 未分类兜底行查询失败（第二次 incident_categories 查询）
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	require.ErrorIs(t, s.DeleteCategory(ctx, tmp.ID), errFailpoint)

	// 事务内 incidents 归并失败 → 回滚
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "update", n: 1, err: errFailpoint})
	require.ErrorIs(t, s.DeleteCategory(ctx, tmp.ID), errFailpoint)

	// 事务内 bugs 归并失败 → 回滚
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

// 软删行占坑：FindByKey（软删过滤）miss → Create 撞真实唯一索引 →
// isUniqueViolation 命中 → 冲突恢复查询仍 miss → 显式 conflict。
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

// Details 更新（map 更新路径绕过 JSON 序列化器）当前会触发
// 「unsupported type map[string]interface {}」——生产缺陷（另录 todo），
// 本测试钉住该行为：缺陷修复后此断言应改为成功落库。
func TestUpdateIncident_DetailsMapUpdateDefectPinned(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "d", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{
		Details: map[string]interface{}{"zone": "cn"},
	})
	require.ErrorContains(t, err, "unsupported type")
}

func TestUpdateIncident_UpdateErrorAndReGetError(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "u", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	// Update 本体失败
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "update", n: 1, err: errFailpoint})
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{GameID: strPtr("g1")})
	require.ErrorIs(t, err, errFailpoint)

	// Update 成功后回读失败（Get 初读为第 1 次，回读为第 2 次）
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

// ---- external.go 错误/守卫臂 ----

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
