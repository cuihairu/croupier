package task

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Category:    "test",
		Metadata: map[string]interface{}{
			"category":      "test",
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
