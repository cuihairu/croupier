package schedule

// service.go 补测（failpoint/守卫口径）：先前零覆盖臂——Create 的
// GovernInvocability 失败透传（禁用函数拦截）、SetStatus active 分支的
// FindByID 失败臂与回读失败臂、TriggerNow 的 FindByID 失败臂与政策审批
// 待审臂（高危契约 → single_admin）、metadata 合并块（含与内建键冲突的
// 跳过臂）、RunLogs 的 ListRunLogs 失败臂、requireScheduleScope 的
// 调度模型未初始化臂与 env 不匹配臂。
//
// 诚实跳过：无。

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// dbFailpoint 按表内第 n 次查询/更新注入确定性错误（每个测试独享 DB）。
type dbFailpoint struct {
	table string
	op    string // query / update
	n     int
	err   error
}

func applyDBFailpoint(t *testing.T, db *gorm.DB, fp dbFailpoint) {
	t.Helper()
	var hits int64
	reg2 := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != fp.table {
			return
		}
		hits++
		if hits == int64(fp.n) {
			_ = tx.AddError(fp.err)
		}
	}
	name := "test:fail_" + fp.table + "_" + fp.op
	switch fp.op {
	case "query":
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg2))
	case "update":
		require.NoError(t, db.Callback().Update().After("gorm:update").Register(name, reg2))
	default:
		t.Fatalf("unknown failpoint op %q", fp.op)
	}
}

var errSchedFailpoint = fmt.Errorf("failpoint: storage down")

// TestCreate_DisabledFunctionRejected：定义期准入拦截禁用函数（159）。
func TestCreate_DisabledFunctionRejected(t *testing.T) {
	r, db := newTestEnv(t, true)
	require.NoError(t, db.Create(&model.Function{FunctionID: "fn.reset", Status: model.StatusDisabled}).Error)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", createBody)
	require.GreaterOrEqual(t, rec.Code, 400)
}

// SetStatus active 分支的 FindByID 失败（198：scope 查 q1、active 重查 q2）。
func TestSetStatus_ActiveFindByIDError(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	applyDBFailpoint(t, db, dbFailpoint{table: "task_schedules", op: "query", n: 2, err: errSchedFailpoint})
	rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+itoaSched(id)+"/status", `{"status":"active"}`)
	require.GreaterOrEqual(t, rec.Code, 400)
}

// SetStatus 成功后的回读失败（220：scope q1、active q2、回读 q3）。
func TestSetStatus_FinalReadError(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	applyDBFailpoint(t, db, dbFailpoint{table: "task_schedules", op: "query", n: 3, err: errSchedFailpoint})
	rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+itoaSched(id)+"/status", `{"status":"active"}`)
	require.GreaterOrEqual(t, rec.Code, 400)
}

// TriggerNow 的 FindByID 失败臂（247：scope q1、本体 q2）。
func TestTriggerNow_FindByIDError(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	applyDBFailpoint(t, db, dbFailpoint{table: "task_schedules", op: "query", n: 2, err: errSchedFailpoint})
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+itoaSched(id)+"/trigger")
	require.GreaterOrEqual(t, rec.Code, 400)
}

// newApprovalEnv：政策审批链齐备的环境（默认政策档：high → single_admin
// 需审批），并给 admin 角色绑定 function:invoke（政策链权限要求）。
func newApprovalEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("schedule_appr_%d", atomic.AddUint64(&dbSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	pm, err := policy.NewManager(db, filepath.Join(t.TempDir(), "no-policy-config.yaml"))
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:                db,
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		AdminModel:        model.NewAdminModel(db),
		RoleModel:         model.NewRoleModel(db),
		GameModel:         model.NewGameModel(db),
		FunctionModel:     model.NewFunctionModel(db),
		PermissionService: permission.NewPermissionService(db),
		RegistryStore:     reg.NewStore(),
		PolicyManager:     pm,
		ApprovalsStore:    approvals.NewMemStore(),
		Dispatcher:        dispatch.NewDispatcher(reg.NewStore()),
	}
	seedScheduleAdmin(t, db, "tester")
	perm := &model.Permission{ID: "function:invoke", Name: "调用函数", Resource: "function", Action: "invoke", Category: "schedule-test"}
	require.NoError(t, db.Create(perm).Error)
	var role model.Role
	require.NoError(t, db.Where("name = ?", "admin").First(&role).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}).Error)

	h := NewHandler(NewService(svcCtx))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "tester")
		ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
		c.Request = c.Request.WithContext(ctx)
	})
	g := r.Group("/api/v1/schedules")
	g.POST("", h.Create)
	g.POST("/:id/trigger", h.TriggerNow)
	return r, db
}

// TriggerNow 政策审批臂：高危契约（risk=high → 默认 high 政策 RequireApproval
// → single_admin）→ 创建审批、不派发（263-267）。
func TestTriggerNow_ApprovalRequired(t *testing.T) {
	r, db := newApprovalEnv(t)
	id := mustCreate(t, r)
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo", Env: "prod", FunctionID: "fn.reset",
		Version: "1.0.0", Enabled: true, Risk: dbenum.RiskHigh,
	}).Error)

	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+itoaSched(id)+"/trigger")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"approvalId":"func_fn.reset_`, "高危函数须返回待审响应")
	require.Contains(t, rec.Body.String(), `"approvalWorkflow":"single_admin"`, "默认 high 政策走 single_admin 工作流")
	require.Contains(t, rec.Body.String(), `"taskRunId":""`, "待审响应不派发")
}

// TriggerNow 的 metadata 合并块：行内 metadata 与内建 meta 键合并，
// 冲突键（env）跳过、新键（custom）合入（276-282 两臂）。
func TestTriggerNow_MetadataMerged(t *testing.T) {
	store := reg.NewStore()
	d := dispatch.NewDispatcher(store)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-meta", GameID: "demo", Env: "prod",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"fn.reset": {Enabled: true}},
	}))
	respBody, err := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: "task-meta-1"})
	require.NoError(t, err)
	d.SetSessionResolver(&stubSessionResolver{caller: &stubSessionCaller{respBody: respBody}})

	r, db := newTestEnvWithDispatcher(t, d)
	row := &model.TaskSchedule{
		Name: "meta", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod",
		FunctionID: "fn.reset", Payload: model.JSON(`{"k":1}`),
		Metadata: model.JSON(`{"custom":"x","env":"meta-env"}`),
		Status:   "active", Kind: "function",
	}
	require.NoError(t, db.Create(row).Error)

	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+itoaSched(row.ID)+"/trigger")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "task-meta-1")
}

// RunLogs 的 ListRunLogs 失败臂（315）：清单走 task_schedule_run_logs 表，
// 第 1 次查询即 Count。
func TestRunLogs_ListError(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	applyDBFailpoint(t, db, dbFailpoint{table: "task_schedule_run_logs", op: "query", n: 1, err: errSchedFailpoint})
	rec := doReq(r, http.MethodGet, "/api/v1/schedules/"+itoaSched(id)+"/runs")
	require.GreaterOrEqual(t, rec.Code, 400)
}

// requireScheduleScope 的调度模型未初始化臂（345）。
func TestRequireScheduleScope_NilModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	err := s.requireScheduleScope(context.Background(), 1)
	require.ErrorContains(t, err, "调度模型未初始化")
}

// requireScheduleScope 的 env 不匹配臂（356）：跨 env 访问按 NotFound 折叠。
func TestRequireScheduleScope_EnvMismatch(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	s := NewService(&svc.ServiceContext{TaskScheduleModel: model.NewTaskScheduleModel(db)})
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	err := s.requireScheduleScope(ctx, id)
	require.ErrorContains(t, err, "not found in env dev")
}

// ---- helpers ----

func itoaSched(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
