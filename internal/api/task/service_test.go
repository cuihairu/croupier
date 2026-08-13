package task

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// setupSvcCtx builds a ServiceContext with everything task.Service.Start needs:
// function lookup, admin/role loading, and a dispatcher wired to an empty
// registry (so dispatches fail with a routing error rather than hitting a network).
func setupSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db := setupTestDB(t)
	regStore := registry.NewStore()
	return &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		AdminModel:    model.NewAdminModel(db),
		RegistryStore: regStore,
		Dispatcher:    dispatch.NewDispatcher(regStore),
	}
}

func createTestFunction(t *testing.T, db *gorm.DB, functionID, name string) *model.Function {
	t.Helper()
	fn := &model.Function{
		FunctionID:  functionID,
		Name:        name,
		Description: "Test function",
		GameID:      "test-game",
		Status:      1,
		Version:     "1.0.0",
		Metadata: map[string]interface{}{
			"version":       "1.0.0",
			"input_schema":  map[string]interface{}{"type": "object"},
			"output_schema": map[string]interface{}{"type": "object"},
		},
	}
	require.NoError(t, db.Create(fn).Error)
	return fn
}

// seedAdminWithRole inserts an admin row bound to a role with the given name and
// returns a context carrying the username (mirroring what the auth middleware does).
func seedAdminWithRole(t *testing.T, db *gorm.DB, username, roleName string) context.Context {
	t.Helper()
	role := &model.Role{Name: roleName, Category: "test"}
	require.NoError(t, db.Create(role).Error)
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, db.Create(admin).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	return context.WithValue(context.Background(), "username", username)
}

func seedTaskRun(t *testing.T, db *gorm.DB, taskID, functionID, status string) *model.TaskRun {
	t.Helper()
	run := &model.TaskRun{
		TaskID:       taskID,
		FunctionID:   functionID,
		Status:       status,
		InputPayload: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, db.Create(run).Error)
	return run
}

// --- Start ---

func TestStart_EmptyFunctionID(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Start(context.Background(), &StartRequest{FunctionID: "  "})
	require.Error(t, err)
}

func TestStart_FunctionNotFound(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Start(context.Background(), &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
}

// TestStart_DispatchReached is the proof-of-fix: with the function registered
// and an admin caller, Start now reaches the dispatcher. An empty registry has
// no agent, so the dispatcher returns a routing error. Before the fix this path
// returned a {status:"queued"} success and stranded a dead row.
func TestStart_DispatchReached_AdminRole(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	ctx := seedAdminWithRole(t, svcCtx.DB, "root", "admin")
	svc := NewService(svcCtx)

	resp, err := svc.Start(ctx, &StartRequest{FunctionID: "player.ban", Params: map[string]interface{}{"id": 1}})
	require.Error(t, err, "dispatch must be attempted; empty registry yields a routing error")
	assert.Nil(t, resp)

	// No stranded queued row: the dispatcher returns before creating a task_runs row.
	var count int64
	require.NoError(t, svcCtx.DB.Model(&model.TaskRun{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no task_runs row should be created when dispatch fails to find an agent")
}

// TestStart_NonAdminRequiresGameScope proves the RBAC/scope gate now runs on
// /tasks (previously Start had no authorization at all).
func TestStart_NonAdminRequiresGameScope(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	ctx := seedAdminWithRole(t, svcCtx.DB, "alice", "operator")
	svc := NewService(svcCtx)

	_, err := svc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
}

// --- List / Detail / Events / Cancel ---

func TestList_FiltersByFunction(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-1", "player.ban", tasks.StatusSucceeded)
	seedTaskRun(t, svcCtx.DB, "t-2", "player.kick", tasks.StatusRunning)
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &ListRequest{FunctionID: "player.ban", Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Items))
	assert.Equal(t, "t-1", resp.Items[0].ID)
}

func TestDetail_NotFound(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Detail(context.Background(), &DetailRequest{ID: "missing"})
	require.Error(t, err)
}

func TestDetail_Found(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-9", "player.ban", tasks.StatusRunning)
	svc := NewService(svcCtx)

	resp, err := svc.Detail(context.Background(), &DetailRequest{ID: "t-9"})
	require.NoError(t, err)
	assert.Equal(t, "t-9", resp.ID)
	assert.Equal(t, "player.ban", resp.FunctionID)
	assert.Equal(t, tasks.StatusRunning, resp.Status)
}

func TestEvents_ReturnsItemsAndAdvancesSeq(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-ev", "player.ban", tasks.StatusSucceeded)
	require.NoError(t, svcCtx.DB.Create(&model.TaskEvent{
		TaskID: "t-ev", Seq: 1, Type: string(tasks.EventStarted), Message: "started",
	}).Error)
	require.NoError(t, svcCtx.DB.Create(&model.TaskEvent{
		TaskID: "t-ev", Seq: 2, Type: string(tasks.EventCompleted), Message: "done",
	}).Error)
	svc := NewService(svcCtx)

	resp, err := svc.Events(context.Background(), &EventsRequest{ID: "t-ev"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Items))
	assert.Equal(t, int64(3), resp.NextSeq, "nextSeq should advance past the last seq")
	assert.True(t, resp.Done, "succeeded run should report done")
}

func TestCancel_MarksCancelRequested(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-cancel", "player.ban", tasks.StatusRunning)
	svc := NewService(svcCtx)

	require.NoError(t, svc.Cancel(context.Background(), &CancelRequest{ID: "t-cancel"}))

	var run model.TaskRun
	require.NoError(t, svcCtx.DB.Where("task_id = ?", "t-cancel").First(&run).Error)
	assert.Equal(t, tasks.StatusCancelRequested, run.Status)

	var events []model.TaskEvent
	require.NoError(t, svcCtx.DB.Where("task_id = ?", "t-cancel").Find(&events).Error)
	found := false
	for _, e := range events {
		if e.Type == string(tasks.EventCancelRequested) {
			found = true
		}
	}
	assert.True(t, found, "a cancel_requested event should be appended")
}

// --- End-to-end dispatch loop ---

// fakeAgentSessionCaller stands in for a real agent's established TCP session.
// On MsgStartTaskRequest it echoes back the server-generated task_id from the
// request metadata, which is exactly what a real agent does to acknowledge the task.
type fakeAgentSessionCaller struct {
	sawFunctionID string
	handledTaskID string
}

func (f *fakeAgentSessionCaller) Call(_ context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	if msgID != protocol.MsgStartTaskRequest {
		return 0, nil, fmt.Errorf("unexpected message id 0x%x", msgID)
	}
	var req sdkv1.InvokeRequest
	if err := proto.Unmarshal(reqBody, &req); err != nil {
		return 0, nil, err
	}
	f.sawFunctionID = req.GetFunctionId()
	if req.Metadata != nil {
		f.handledTaskID = req.Metadata["task_id"]
	}
	resp, err := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: f.handledTaskID})
	if err != nil {
		return 0, nil, err
	}
	return protocol.MsgStartTaskResponse, resp, nil
}

type fakeAgentResolver struct{ caller *fakeAgentSessionCaller }

func (r *fakeAgentResolver) ResolveAgentConn(_ string) (transport.SessionCaller, bool) {
	return r.caller, true
}

// TestStart_DispatchesAndPersists_HappyPath is the end-to-end proof that
// /tasks Start now drives the dispatch loop: it reaches the agent over the
// session, the agent acknowledges with the server-generated task id, and a
// task_runs row is persisted keyed by that id. Before the fix this loop did not
// exist; no test previously covered the happy path.
func TestStart_DispatchesAndPersists_HappyPath(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)

	// Wire the dispatcher to persist task_runs into the same DB the service
	// reads from, and to reach a fake agent session instead of the network.
	svcCtx.Dispatcher.SetTaskRunWriter(dispatch.NewTaskRunWriterAdapter(model.NewTaskRunModel(svcCtx.DB)))
	caller := &fakeAgentSessionCaller{}
	svcCtx.Dispatcher.SetSessionResolver(&fakeAgentResolver{caller: caller})

	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	// Register a live agent that serves the function.
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-e2e",
		GameID:   "test-game",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]registry.FunctionMeta{
			"player.ban": {Enabled: true},
		},
	})

	ctx := seedAdminWithRole(t, svcCtx.DB, "root", "admin")
	svc := NewService(svcCtx)

	resp, err := svc.Start(ctx, &StartRequest{
		FunctionID: "player.ban",
		Params:     map[string]interface{}{"player": "p1"},
		GameID:     "test-game",
		Env:        "prod",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.TaskID, "server should return the dispatched task id")
	assert.Equal(t, tasks.StatusDispatching, resp.Status)

	// The agent received the StartTaskRequest for the right function, carrying
	// the same server-generated task id.
	assert.Equal(t, "player.ban", caller.sawFunctionID)
	assert.Equal(t, resp.TaskID, caller.handledTaskID)

	// A task_runs row was persisted keyed by that id — the feedback loop is closed:
	// events coming back from the agent can now match and update this row.
	var run model.TaskRun
	require.NoError(t, svcCtx.DB.Where("task_id = ?", resp.TaskID).First(&run).Error)
	assert.Equal(t, "player.ban", run.FunctionID)
	assert.Equal(t, "agent-e2e", run.AgentID)
	assert.Equal(t, "test-game", run.GameID)
	assert.Equal(t, "prod", run.Env)
}

// --- DurationMs calculation and Actor/Addr fields ---

func TestBuildItem_CalculatesDurationMs(t *testing.T) {
	t.Parallel()
	startedAt := time.Now().Add(-5 * time.Second)
	finishedAt := time.Now()

	run := &model.TaskRun{
		TaskID:     "t-dur",
		FunctionID: "player.ban",
		Status:     tasks.StatusSucceeded,
		Actor:      "admin",
		Addr:       "192.168.1.100:9090",
		TraceID:    "trace-123",
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	}

	item := buildItem(run)

	assert.Equal(t, "t-dur", item.ID)
	assert.Equal(t, "admin", item.Actor)
	assert.Equal(t, "192.168.1.100:9090", item.Addr)
	assert.Equal(t, "trace-123", item.TraceID)
	assert.NotEmpty(t, item.StartedAt)
	assert.NotEmpty(t, item.FinishedAt)
	assert.Greater(t, item.DurationMs, int64(0), "DurationMs should be positive")
	assert.Less(t, item.DurationMs, int64(10000), "DurationMs should be less than 10s")
}

func TestBuildItem_ZeroDurationMsWhenNoTimes(t *testing.T) {
	t.Parallel()

	run := &model.TaskRun{
		TaskID:     "t-no-time",
		FunctionID: "player.ban",
		Status:     tasks.StatusRunning,
		Actor:      "admin",
	}

	item := buildItem(run)

	assert.Equal(t, "admin", item.Actor)
	assert.Empty(t, item.StartedAt)
	assert.Empty(t, item.FinishedAt)
	assert.Equal(t, int64(0), item.DurationMs, "DurationMs should be 0 when no start/finish times")
}

func TestBuildDetail_IncludesAllFields(t *testing.T) {
	t.Parallel()
	startedAt := time.Now().Add(-3 * time.Second)
	finishedAt := time.Now()

	run := &model.TaskRun{
		TaskID:     "t-detail",
		FunctionID: "player.kick",
		Status:     tasks.StatusSucceeded,
		Actor:      "root",
		Addr:       "10.0.0.1:19090",
		TraceID:    "trace-456",
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	}

	detail := buildDetail(run)

	assert.Equal(t, "t-detail", detail.ID)
	assert.Equal(t, "root", detail.Actor)
	assert.Equal(t, "10.0.0.1:19090", detail.Addr)
	assert.Equal(t, "trace-456", detail.TraceID)
	assert.NotEmpty(t, detail.StartedAt)
	assert.NotEmpty(t, detail.FinishedAt)
	assert.Greater(t, detail.DurationMs, int64(0))
}

func TestList_ReturnsActorAndAddr(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)

	// Create a task run with Actor and Addr
	startedAt := time.Now().Add(-2 * time.Second)
	finishedAt := time.Now()
	run := &model.TaskRun{
		TaskID:       "t-actor",
		FunctionID:   "player.ban",
		Status:       tasks.StatusSucceeded,
		Actor:        "testuser",
		Addr:         "192.168.1.200:9090",
		TraceID:      "trace-789",
		InputPayload: datatypes.JSON([]byte("{}")),
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
	}
	require.NoError(t, svcCtx.DB.Create(run).Error)

	svc := NewService(svcCtx)
	resp, err := svc.List(context.Background(), &ListRequest{FunctionID: "player.ban", Page: 1, Size: 10})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Items))

	item := resp.Items[0]
	assert.Equal(t, "testuser", item.Actor)
	assert.Equal(t, "192.168.1.200:9090", item.Addr)
	assert.Equal(t, "trace-789", item.TraceID)
	assert.Greater(t, item.DurationMs, int64(0))
}

func TestDetail_ReturnsActorAndAddr(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)

	// Create a task run with Actor and Addr
	run := &model.TaskRun{
		TaskID:       "t-detail-actor",
		FunctionID:   "player.ban",
		Status:       tasks.StatusRunning,
		Actor:        "admin",
		Addr:         "10.0.0.50:19090",
		TraceID:      "trace-abc",
		InputPayload: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, svcCtx.DB.Create(run).Error)

	svc := NewService(svcCtx)
	detail, err := svc.Detail(context.Background(), &DetailRequest{ID: "t-detail-actor"})
	require.NoError(t, err)

	assert.Equal(t, "admin", detail.Actor)
	assert.Equal(t, "10.0.0.50:19090", detail.Addr)
	assert.Equal(t, "trace-abc", detail.TraceID)
}
