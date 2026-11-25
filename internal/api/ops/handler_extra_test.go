package ops

// Extra coverage for handler.go: supervisor-family handler error branches and
// the log download default filename (filename omitted + X-Truncated=false),
// the metrics/processes success surface, alias delegation (AgentSupervisor
// family / AgentProcessUpdate), the process-update success path that reads path
// parameters directly, the alert handling handler round trip, empty-store
// success and bind errors for metrics history / log search, the five batch-op
// endpoints (create with scope injection and real name, execute dry-run,
// detail, list, error branches), the eight runbook endpoints (define with
// scope/real name, list, detail, delete 204, run (a ContinueOnFailure step →
// partial still 200), run detail and list), and service error branches
// (supervisor/process update with an unreachable tunnel, ghost agent, and
// batch op / runbook with an unwired orchestrator).
//
// Honestly skipped (unreachable): the GET-side handler bind error branches —
// the three supervisor endpoints (110/129/150), OpsNodeDetail (493),
// BatchOpList (1013), RunbookList (1059), RunbookRunList (1127). No request
// field carries binding:required, and BindQueryCompat swallows
// ShouldBindQuery errors (#59, same class), so the GET side cannot trigger them.

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

// bareOpsHandler is a handler whose service is completely empty (the error branch of each handler).
func bareOpsHandler() *Handler { return NewHandler(NewService(&svc.ServiceContext{})) }

// scopedOpsCtx builds a test context carrying the game/env scope and an optional
// username (with path parameters).
func scopedOpsCtx(method, target, body, username string, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, nil)
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	ctx := svc.WithGameScope(req.Context(), svc.GameScope{GameID: "demo", Env: "prod"})
	if username != "" {
		ctx = context.WithValue(ctx, "username", username) //nolint:staticcheck // matches the key contract of utils.CurrentUsername
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

	// missing registry store → requireAgentScope error → the error branch of each handler
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

	// the alias delegation shares the same error surface (covering the delegation line itself)
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

	// metrics / processes success surface (metrics echoes agentId, processes
	// returns only the process table)
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

	// supervisor log success surface: filename omitted → default name + X-Truncated=false
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

	// alias AgentSupervisorLog delegation
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

	// bad JSON → 400 (bind branch)
	ctx, w := newOpsTestContext(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update", "{bad")
	ctx.Params = params
	h.OpsAgentProcessUpdate(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// success: stagedPath/expectedSha256 come from the body, agentId/name from the path
	ctx, w = newOpsTestContext(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update",
		`{"stagedPath":"/tmp/staged","expectedSha256":"sha"}`)
	ctx.Params = params
	h.OpsAgentProcessUpdate(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "fp-9")

	// alias AgentProcessUpdate delegation
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

	// bad JSON → 400
	ctx, w := newOpsTestContext(http.MethodPost, "/api/v1/ops/alerts/1/handle", "{bad")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	h.OpsAlertHandle(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// nonexistent alert → error branch
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/alerts/99/handle", `{"action":"acknowledge"}`, "alice",
		gin.Param{Key: "id", Value: "99"})
	h.OpsAlertHandle(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// successful transition (handleStatus → acknowledged)
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

	// metrics history: MetricsStore missing → empty success
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=a1", "")
	h.AgentMetricsHistory(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// log search: agentId missing → bind validation error
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/logs", "")
	h.AgentLogSearch(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// log search: store missing → empty success
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

	// create: bad JSON → 400
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch", "{bad", "alice")
	h.BatchOpCreateHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// create success: scope injection + real name (no gameId/env in the body, ctx demo/prod)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch",
		`{"kind":"config","agentIds":["a1"],"config":{"path":"/etc/x","contentBase64":"djE=","mode":644}}`, "alice")
	h.BatchOpCreateHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	var created model.BatchOp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "demo", created.GameID, "empty gameId must be filled from authenticated scope")
	require.Equal(t, "alice", created.CreatedBy)
	batchID := created.BatchID

	// execute: bad JSON / missing-batch error
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/b1/execute", "{bad", "alice",
		gin.Param{Key: "id", Value: "b1"})
	h.BatchOpExecuteHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/ghost/execute", `{"dryRun":true}`, "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.BatchOpExecuteHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// execute dry-run success
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/batch/"+batchID+"/execute", `{"dryRun":true}`, "alice",
		gin.Param{Key: "id", Value: batchID})
	h.BatchOpExecuteHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// detail: missing error / hit success
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch/ghost", "", "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.BatchOpGetHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch/"+batchID, "", "alice",
		gin.Param{Key: "id", Value: batchID})
	h.BatchOpGetHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// list
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/batch?limit=10", "", "alice")
	h.BatchOpListHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), batchID)
}

func TestOpsHandlerRunbookFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := runbookDB(t)
	h := NewHandler(NewService(&svc.ServiceContext{DB: db}))

	// define: bad JSON → 400
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks", "{bad", "alice")
	h.RunbookCreateHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// define success (scope injection + real name)
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

	// define validation failure (name missing) → error branch
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks", `{"description":"no name"}`, "alice")
	h.RunbookCreateHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// list
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks?limit=10", "", "alice")
	h.RunbookListHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), rbID)

	// detail: missing error / hit
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks/ghost", "", "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.RunbookGetHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = scopedOpsCtx(http.MethodGet, "/api/v1/ops/runbooks/"+rbID, "", "alice",
		gin.Param{Key: "id", Value: rbID})
	h.RunbookGetHandler(ctx)
	require.Equal(t, http.StatusOK, w.Code)

	// delete: missing error / success 204 (insert one first, then delete it, keeping the list sample)
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
	// gin's c.Status defers setting the status code, so the test context must read
	// the Writer's cached value
	require.Equal(t, http.StatusNoContent, ctx.Writer.Status())

	// run: bad JSON → 400; missing runbook → error
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks/"+rbID+"/run", "{bad", "alice",
		gin.Param{Key: "id", Value: rbID})
	h.RunbookRunHandler(ctx)
	require.Equal(t, http.StatusBadRequest, w.Code)
	ctx, w = scopedOpsCtx(http.MethodPost, "/api/v1/ops/runbooks/ghost/run", `{}`, "alice",
		gin.Param{Key: "id", Value: "ghost"})
	h.RunbookRunHandler(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// run: a batch_exec step with no registry (batch-op orchestrator
	// unavailable) → step fails, but continueOnFailure → partial final state and
	// HTTP 200 (success branch)
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

	// run detail / run list
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

	// a session is registered but the ops tunnel is unreachable (no resolver) →
	// supervisor-family service error branches (110/129/150)
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

	// ghost agent → requireAgentScope "agent not found" → the service error
	// branches of metrics history and log search (493/766)
	hGhost := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStore()}))
	ctx, w := newOpsTestContext(http.MethodGet, "/api/v1/ops/x?agentId=ghost", "")
	hGhost.AgentMetricsHistory(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
	ctx, w = newOpsTestContext(http.MethodGet, "/api/v1/ops/logs?agentId=ghost", "")
	hGhost.AgentLogSearch(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)

	// orchestrator not wired → batch-op create/execute/list service error branches (975/1013/1018)
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

	// meta DB not wired → runbook create/list/run-list service error branches
	// (1059/1064/1132)
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

	// a session is registered but the tunnel is unreachable (no resolver) → service error branch (254)
	ctx, w := scopedOpsCtx(http.MethodPost, "/api/v1/ops/agents/a1/processes/game/update",
		`{"stagedPath":"/tmp/staged","expectedSha256":"sha"}`, "alice",
		gin.Param{Key: "agentId", Value: "a1"}, gin.Param{Key: "name", Value: "game"})
	h.OpsAgentProcessUpdate(ctx)
	require.NotEqual(t, http.StatusOK, w.Code)
}
