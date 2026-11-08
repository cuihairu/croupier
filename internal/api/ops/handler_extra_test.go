package ops

// handler.go 补测：supervisor 族 handler 错误分支与日志下载默认文件名
// （文件名缺省 + X-Truncated=false）、metrics/processes 成功面、别名委托
// （AgentSupervisor 族 / AgentProcessUpdate）、路径参数直取的进程更新成功、
// alert 流转 handler 往返、metrics history/log search 的空 store 成功与
// bind 错误、batch op 五端点（创建带 scope 注入与实名、执行 dry-run、详情、
// 列表、错误分支）、runbook 八端点（定义带 scope/实名、列表、详情、删除
// 204、运行（ContinueOnFailure 步 → partial 仍 200）、运行详情与列表）、
// service 错误分支（隧道不可达的 supervisor/进程更新、ghost agent、
// 编排器未装配的 batch op 与 runbook）。
//
// 诚实跳过（不可达）：GET 侧 handler 的 bind 错误分支——supervisor 三端点
// （110/129/150）、OpsNodeDetail（493）、BatchOpList（1013）、RunbookList
// （1059）、RunbookRunList（1127）。请求字段均无 binding:required，且
// BindQueryCompat 吞掉 ShouldBindQuery 错误（#59 同款），GET 侧不可触发。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// bareOpsHandler service 全空的 handler（各 handler 的错误分支）。
func bareOpsHandler() *Handler { return NewHandler(NewService(&svc.ServiceContext{})) }

// scopedOpsCtx 挂 game/env scope 与可选用户名的测试上下文（带路径参数）。
func scopedOpsCtx(method, target, body, username string, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, nil)
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	ctx := svc.WithGameScope(req.Context(), svc.GameScope{GameID: "demo", Env: "prod"})
	if username != "" {
		ctx = context.WithValue(ctx, "username", username) //nolint:staticcheck // 与 utils.CurrentUsername 键契约一致
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = params
	return c, w
}

func TestOpsHandlerSupervisorFamilyErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := bareOpsHandler()

	// registry store 缺失 → requireAgentScope 错误 → 各 handler 错误分支
	for _, tc := range []struct {
		name string
		call func(*gin.Context)
	}{
		{"supervisor", h.OpsAgentSupervisor},
		{"supervisor-events", h.OpsAgentSupervisorEvents},
		{"supervisor-log", h.OpsAgentSupervisorLog},
		{"metrics", h.OpsAgentMetrics},
		{"processes", h.OpsAgentProcesses},
	} {
		ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
		tc.call(ctx)
		require.NotEqual(t, http.StatusOK, w.Code, tc.name)
	}

	// 别名委托同错误面（覆盖委托行本身）
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.AgentSupervisor(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.AgentSupervisorEvents(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestOpsHandlerMetricsProcessesAndLogSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	metrics := registry.NewMetricsStore()
	metrics.Add("a1", &opsv1.MetricsReport{
		Networks: []*opsv1.NetworkMetrics{nil, {Interface: "eth0"}},
		SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
			nil,
			{Name: "game", Pid: 7, State: opsv1.ProcessState_PROCESS_STATE_RUNNING, RestartCount: 2},
		},
	})
	svcCtx := &svc.ServiceContext{RegistryStore: store, MetricsStore: metrics}
	h := NewHandler(NewService(svcCtx))

	// metrics / processes 成功面（metrics 回显 agentId，processes 只回进程表）
	for _, tc := range []struct {
		name     string
		contains string
		call     func(*gin.Context)
	}{
		{"metrics", "a1", h.OpsAgentMetrics},
		{"processes", "game", h.OpsAgentProcesses},
	} {
		ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
		tc.call(ctx)
		require.Equal(t, http.StatusOK, w.Code, tc.name)
		require.Contains(t, w.Body.String(), tc.contains)
	}

	// supervisor log 成功面：文件名缺省 → 默认名 + X-Truncated=false
	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		uint32(protocol.MsgGetSupervisorLogRequest): &opsv1.GetSupervisorLogResponse{Content: []byte("line-1\n")},
	}}
	setupOpsResolver(t, caller)
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.OpsAgentSupervisorLog(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `attachment; filename="supervisor-events.log"`, w.Header().Get("Content-Disposition"))
	require.Equal(t, "false", w.Header().Get("X-Truncated"))
	require.Contains(t, w.Body.String(), "line-1")

	// 别名 AgentSupervisorLog 委托
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.AgentSupervisorLog(ctx)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOpsHandlerAgentProcessUpdateSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	h := NewHandler(NewService(svcCtx))

	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		uint32(protocol.MsgUpdateProcessRequest): &opsv1.UpdateProcessResponse{Success: true, NewFingerprint: "fp-9"},
	}}
	setupOpsResolver(t, caller)

	params := gin.Params{{Key: "agentId", Value: "a1"}, {Key: "name", Value: "game"}}

	// 坏 JSON → 400（bind 分支）
	ctx, w := newOpsTestContext(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update", "{bad")
	ctx.Params = params
	h.OpsAgentProcessUpdate(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 成功：stagedPath/expectedSha256 走 body，agentId/name 走路径
	ctx, w = newOpsTestContext(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update",
		`{"stagedPath":"/tmp/staged","expectedSha256":"sha"}`)
	ctx.Params = params
	h.OpsAgentProcessUpdate(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "fp-9")

	// 别名 AgentProcessUpdate 委托
	ctx, w = newOpsTestContext(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update",
		`{"stagedPath":"/tmp/staged","expectedSha256":"sha"}`)
	ctx.Params = params
	h.AgentProcessUpdate(ctx)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOpsHandlerAlertHandleRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/ah.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Alert{}))
	require.NoError(t, model.NewAlertModel(db).Create(context.Background(), &model.Alert{
		Level: "warn", Message: "cpu high", GameID: "demo", Env: "prod",
	}))
	h := NewHandler(NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)}))

	// 坏 JSON → 400
	ctx, w := newOpsTestContext(http.MethodPost, "/api/v1/ops/alerts/1/handle", "{bad")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	h.OpsAlertHandle(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 不存在的告警 → 错误分支
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/alerts/99/handle", `{"action":"acknowledge"}`, "alice",
		gin.Param{Key: "id", Value: "99"})
	h.OpsAlertHandle(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// 成功流转（handleStatus → acknowledged）
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/alerts/1/handle",
		`{"action":"acknowledge","remark":"seen"}`, "alice",
		gin.Param{Key: "id", Value: "1"})
	h.OpsAlertHandle(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), model.AlertHandleAcknowledged)
}

func TestOpsHandlerMetricsHistoryAndLogSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))

	// metrics history：MetricsStore 缺失 → 空成功
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.AgentMetricsHistory(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// log search：agentId 缺失 → bind 校验错误
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/logs", "")
	h.AgentLogSearch(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// log search：store 缺失 → 空成功
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/logs?agentId=a1", "")
	h.AgentLogSearch(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "a1")
}

func TestOpsHandlerBatchOpFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/bo.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BatchOp{}))
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	h := NewHandler(NewService(&svc.ServiceContext{DB: db, RegistryStore: store}))

	// 创建：坏 JSON → 400
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch", "{bad", "alice")
	h.BatchOpCreateHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 创建成功：scope 注入 + 实名（body 不带 gameId/env，ctx demo/prod）
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch",
		`{"kind":"config","agentIds":["a1"],"config":{"path":"/etc/x","contentBase64":"djE=","mode":644}}`, "alice")
	h.BatchOpCreateHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	var created model.BatchOp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "demo", created.GameID, "empty gameId must be filled from authenticated scope")
	require.Equal(t, "alice", created.CreatedBy)
	batchID := created.BatchID

	// 执行：坏 JSON / 缺失批次错误
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/b1/execute", "{bad", "alice",
		gin.Param{Key: "id", Value: "b1"})
	h.BatchOpExecuteHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/ghost/execute", `{"dryRun":true}`, "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.BatchOpExecuteHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// 执行 dry-run 成功
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/"+batchID+"/execute", `{"dryRun":true}`, "alice",
		gin.Param{Key: "id", Value: batchID})
	h.BatchOpExecuteHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// 详情：缺失错误 / 命中成功
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch/ghost", "", "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.BatchOpGetHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch/"+batchID, "", "alice",
		gin.Param{Key: "id", Value: batchID})
	h.BatchOpGetHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// 列表
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch?limit=10", "", "alice")
	h.BatchOpListHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), batchID)
}

func TestOpsHandlerRunbookFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runbookDB(t)
	h := NewHandler(NewService(&svc.ServiceContext{DB: db}))

	// 定义：坏 JSON → 400
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks", "{bad", "alice")
	h.RunbookCreateHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 定义成功（scope 注入 + 实名）
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks",
		`{"name":"restart","description":"dr","steps":[{"name":"s1","kind":"batch_exec","agentIds":["a1"],"exec":{"command":"echo"},"continueOnFailure":true}]}`, "alice")
	h.RunbookCreateHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	var created model.Runbook
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "restart", created.Name)
	require.Equal(t, "alice", created.CreatedBy)
	rbID := created.RunbookID
	require.NotEmpty(t, rbID, "Define must mint a stable runbook id")

	// 定义校验失败（name 缺失）→ 错误分支
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks", `{"description":"no name"}`, "alice")
	h.RunbookCreateHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// 列表
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks?limit=10", "", "alice")
	h.RunbookListHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), rbID)

	// 详情：缺失错误 / 命中
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks/ghost", "", "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.RunbookGetHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks/"+rbID, "", "alice",
		gin.Param{Key: "id", Value: rbID})
	h.RunbookGetHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// 删除：缺失错误 / 成功 204（先复制一条再删，保留列表样例）
	ctx, w = scopedOpsCtx(http.MethodDelete, "/api/v1/ops/runbooks/ghost", "", "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.RunbookDeleteHandler(ctx)
	require.NotEqual(t, http.StatusNoContent, ctx.Writer.Status())
	require.NoError(t, model.NewRunbookModel(db).Create(context.Background(), &model.Runbook{
		RunbookID: "rb-tmp", Name: "tmp", GameID: "demo", Env: "prod", StepsJSON: "[]",
	}))
	ctx, w = scopedOpsCtx(http.MethodDelete, "/api/v1/ops/runbooks/rb-tmp", "", "alice",
		gin.Param{Key: "id", Value: "rb-tmp"})
	h.RunbookDeleteHandler(ctx)
	// gin 的 c.Status 延迟落状态码，测试上下文须读 Writer 缓存值
	require.Equal(t, http.StatusNoContent, ctx.Writer.Status())

	// 运行：坏 JSON → 400；runbook 缺失 → 错误
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks/"+rbID+"/run", "{bad", "alice",
		gin.Param{Key: "id", Value: rbID})
	h.RunbookRunHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks/ghost/run", `{}`, "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.RunbookRunHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// 运行：batch_exec 步骤无 registry（batch op 编排器不可用）→ 步失败，
	// 但 continueOnFailure → partial 终态且 HTTP 200（成功分支）
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks/"+rbID+"/run", `{"params":{}}`, "alice",
		gin.Param{Key: "id", Value: rbID})
	h.RunbookRunHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), model.RunbookRunStatusPartial)
	var runRes struct {
		RunID string `json:"runId"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runRes))
	require.NotEmpty(t, runRes.RunID)

	// 运行详情 / 运行列表
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbook-runs/ghost", "", "alice",
		gin.Param{Key: "runId", Value: "ghost"})
	h.RunbookRunGetHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbook-runs/"+runRes.RunID, "", "alice",
		gin.Param{Key: "runId", Value: runRes.RunID})
	h.RunbookRunGetHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbook-runs?runbookId="+rbID+"&limit=10", "", "alice")
	h.RunbookRunListHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), runRes.RunID)
}

func TestOpsHandlerServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))

	// registry 有会话但 ops 隧道不可达（无解析器）→ supervisor 族 service
	// 错误分支（110/129/150）
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))
	for _, tc := range []struct {
		name string
		call func(*gin.Context)
	}{
		{"supervisor", h.OpsAgentSupervisor},
		{"supervisor-events", h.OpsAgentSupervisorEvents},
		{"supervisor-log", h.OpsAgentSupervisorLog},
	} {
		ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
		tc.call(ctx)
		require.NotEqual(t, http.StatusOK, w.Code, tc.name)
	}

	// ghost agent → requireAgentScope "agent not found" → metrics history
	// 与 log search 的 service 错误分支（493/766）
	hGhost := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStore()}))
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=ghost", "")
	hGhost.AgentMetricsHistory(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/logs?agentId=ghost", "")
	hGhost.AgentLogSearch(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// 编排器未装配 → batch op 创建/执行/列表 service 错误分支（975/1013/1018）
	hBatch := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch",
		`{"kind":"config","agentIds":["a1"],"config":{"path":"/x","contentBase64":"djE=","mode":644}}`, "alice")
	hBatch.BatchOpCreateHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/b1/execute", `{"dryRun":true}`, "alice",
		gin.Param{Key: "id", Value: "b1"})
	hBatch.BatchOpExecuteHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch", "", "alice")
	hBatch.BatchOpListHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// meta DB 未装配 → runbook 创建/列表/运行列表 service 错误分支
	// （1059/1064/1132）
	hRb := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks",
		`{"name":"rb","steps":[]}`, "alice")
	hRb.RunbookCreateHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks", "", "alice")
	hRb.RunbookListHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbook-runs", "", "alice")
	hRb.RunbookRunListHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestOpsHandlerProcessUpdateServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))

	// 会话在册但隧道不可达（无解析器）→ service 错误分支（254）
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update",
		`{"stagedPath":"/tmp/staged","expectedSha256":"sha"}`, "alice",
		gin.Param{Key: "agentId", Value: "a1"}, gin.Param{Key: "name", Value: "game"})
	h.OpsAgentProcessUpdate(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
}
