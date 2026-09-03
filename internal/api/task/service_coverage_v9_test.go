package task

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRuntimeV9 是 TaskRuntime 的可配置替身，用于覆盖 service 的错误分支。
type fakeRuntimeV9 struct {
	startErr error
	lastReq  *sdkv1.InvokeRequest

	cancelErr    error
	canceledTask string

	findFn  *model.FunctionContract
	findErr error

	getRun *model.TaskRun
	getErr error

	listRunsErr error

	events        []model.TaskEvent
	listEventsErr error

	updateErr   error
	updateIfOK  bool
	updateIfErr error

	appendErr error
}

func (f *fakeRuntimeV9) StartTask(_ context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	f.lastReq = req
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &sdkv1.StartTaskResponse{TaskId: "fake-task-v9"}, nil
}

func (f *fakeRuntimeV9) CancelTask(_ context.Context, taskID string) error {
	f.canceledTask = taskID
	return f.cancelErr
}

func (f *fakeRuntimeV9) FindFunctionContract(_ context.Context, _, _, _ string) (*model.FunctionContract, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.findFn, nil
}

func (f *fakeRuntimeV9) GetRun(_ context.Context, _ string) (*model.TaskRun, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getRun, nil
}

func (f *fakeRuntimeV9) ListRuns(_ context.Context, _ model.ListTasksOptions) ([]model.TaskRun, int64, error) {
	if f.listRunsErr != nil {
		return nil, 0, f.listRunsErr
	}
	return nil, 0, nil
}

func (f *fakeRuntimeV9) ListEvents(_ context.Context, _ string, _ int64) ([]model.TaskEvent, error) {
	if f.listEventsErr != nil {
		return nil, f.listEventsErr
	}
	return f.events, nil
}

func (f *fakeRuntimeV9) UpdateRun(_ context.Context, _ string, _ map[string]interface{}) error {
	return f.updateErr
}

func (f *fakeRuntimeV9) UpdateRunIfStatusNotIn(_ context.Context, _ string, _ []string, _ map[string]interface{}) (bool, error) {
	if f.updateIfErr != nil {
		return false, f.updateIfErr
	}
	return f.updateIfOK, nil
}

func (f *fakeRuntimeV9) AppendEvent(_ context.Context, _ string, _ tasks.EventType, _ int32, _ string, _ []byte) error {
	return f.appendErr
}

func TestServiceListV9_RuntimeError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{listRunsErr: errors.New("db down")}}

	_, err := taskSvc.List(context.Background(), &ListRequest{Page: 1, Size: 10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestServiceStartV9_FindContractError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findErr: errors.New("contract missing")}}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
}

func TestServiceStartV9_LoadAdminError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	// 带 scope 但不带 username：LoadCurrentAdmin 必须失败。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到登录用户")
}

func TestServiceStartV9_PermissionIDsError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svcCtx.RoleModel = model.NewRoleModel(svcCtx.DB)
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "dave", "operator"), svc.GameScope{GameID: "test-game", Env: "test-env"})
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))

	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
}

func TestServiceStartV9_ScopeCheckerNotInitialized(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "erin", "operator"), svc.GameScope{GameID: "test-game", Env: "test-env"})

	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope checker not initialized")
}

func TestServiceStartV9_InvokePermissionDenied(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svcCtx.RoleModel = model.NewRoleModel(svcCtx.DB)
	svcCtx.GameModel = model.NewGameModel(svcCtx.DB)
	svcCtx.PermissionService = permission.NewPermissionService(svcCtx.DB)
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")

	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "carol", "operator"), svc.GameScope{GameID: "test-game", Env: "test-env"})

	game := &model.Game{Name: "TestGame", GameID: "test-game"}
	require.NoError(t, svcCtx.DB.Create(game).Error)
	var admin model.Admin
	require.NoError(t, svcCtx.DB.Where("username = ?", "carol").First(&admin).Error)
	require.NoError(t, svcCtx.DB.Create(&model.AdminGameScope{AdminID: admin.ID, GameID: game.ID}).Error)

	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestServiceStartV9_NilParamsDefaultPayloadAndMetadata(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	rt := &fakeRuntimeV9{findFn: &model.FunctionContract{}}
	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "root", "admin"), svc.GameScope{GameID: "test-game", Env: "test-env"})

	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}
	resp, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "fake-task-v9", resp.TaskID)
	assert.Equal(t, tasks.StatusDispatching, resp.Status)

	// Params 为 nil 时应补空对象，metadata 记录 scope 与操作者。
	require.NotNil(t, rt.lastReq)
	assert.JSONEq(t, "{}", string(rt.lastReq.GetPayload()))
	assert.Equal(t, "test-game", rt.lastReq.Metadata["gameId"])
	assert.Equal(t, "test-env", rt.lastReq.Metadata["env"])
	assert.Equal(t, "root", rt.lastReq.Metadata["actor"])
}

func TestServiceStartV9_MarshalParamsError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "root", "admin"), svc.GameScope{GameID: "test-game", Env: "test-env"})

	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban", Params: make(chan int)})
	require.Error(t, err)
}

func TestServiceDetailV9_ScopeForbidden(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	run := seedTaskRun(t, svcCtx.DB, "t-scope", "player.ban", tasks.StatusRunning)
	run.GameID = "game-a"
	run.Env = "prod"
	require.NoError(t, svcCtx.DB.Save(run).Error)

	rt := &fakeRuntimeV9{getRun: run}
	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})

	_, err := taskSvc.Detail(ctx, &DetailRequest{ID: "t-scope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该任务")
}

func TestServiceEventsV9_ErrorPaths(t *testing.T) {
	t.Run("get run error", func(t *testing.T) {
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getErr: errors.New("store down")}}
		_, err := taskSvc.Events(context.Background(), &EventsRequest{ID: "t-x"})
		require.Error(t, err)
	})

	t.Run("scope forbidden", func(t *testing.T) {
		run := &model.TaskRun{TaskID: "t-x", GameID: "game-a", Env: "prod"}
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getRun: run}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})
		_, err := taskSvc.Events(ctx, &EventsRequest{ID: "t-x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "无权访问该任务")
	})

	t.Run("nil run forbidden", func(t *testing.T) {
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})
		_, err := taskSvc.Events(ctx, &EventsRequest{ID: "t-x"})
		require.Error(t, err)
	})

	t.Run("list events error", func(t *testing.T) {
		run := &model.TaskRun{TaskID: "t-x", GameID: "game-a", Env: "prod"}
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getRun: run, listEventsErr: errors.New("events down")}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"})
		_, err := taskSvc.Events(ctx, &EventsRequest{ID: "t-x"})
		require.Error(t, err)
	})
}

func TestServiceCancelV9_ErrorPaths(t *testing.T) {
	t.Run("get run error", func(t *testing.T) {
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getErr: errors.New("store down")}}
		err := taskSvc.Cancel(context.Background(), &CancelRequest{ID: "t-x"})
		require.Error(t, err)
	})

	t.Run("scope forbidden", func(t *testing.T) {
		run := &model.TaskRun{TaskID: "t-x", GameID: "game-a", Env: "prod"}
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getRun: run}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})
		err := taskSvc.Cancel(ctx, &CancelRequest{ID: "t-x"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "无权访问该任务")
	})

	t.Run("update run error", func(t *testing.T) {
		run := &model.TaskRun{TaskID: "t-x", GameID: "game-a", Env: "prod"}
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getRun: run, updateIfErr: errors.New("update down")}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"})
		err := taskSvc.Cancel(ctx, &CancelRequest{ID: "t-x"})
		require.Error(t, err)
	})

	t.Run("append event error", func(t *testing.T) {
		run := &model.TaskRun{TaskID: "t-x", GameID: "game-a", Env: "prod"}
		taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: &fakeRuntimeV9{getRun: run, updateIfOK: true, appendErr: errors.New("append down")}}
		ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"})
		err := taskSvc.Cancel(ctx, &CancelRequest{ID: "t-x"})
		require.Error(t, err)
	})
}

func TestServiceCancelV9_ForwardsCancelTask(t *testing.T) {
	run := &model.TaskRun{TaskID: "t-fwd", GameID: "game-a", Env: "prod"}
	rt := &fakeRuntimeV9{getRun: run, updateIfOK: true}
	taskSvc := &Service{svcCtx: setupSvcCtx(t), runtime: rt}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"})

	require.NoError(t, taskSvc.Cancel(ctx, &CancelRequest{ID: "t-fwd"}))
	assert.Equal(t, "t-fwd", rt.canceledTask)
}

func TestRuntimeV9_NilComponents(t *testing.T) {
	rt := &taskRuntime{}

	_, err := rt.StartTask(context.Background(), &sdkv1.InvokeRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task dispatcher not configured")

	require.NoError(t, rt.CancelTask(context.Background(), "t-x"))

	_, err = rt.FindFunctionContract(context.Background(), "g", "e", "f")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function contract model not configured")
}

func TestRuntimeV9_UpdateRun(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-upd", "player.ban", tasks.StatusRunning)
	rt := NewTaskRuntime(svcCtx)

	require.NoError(t, rt.UpdateRun(context.Background(), "t-upd", map[string]interface{}{"message": "v9-updated"}))

	var run model.TaskRun
	require.NoError(t, svcCtx.DB.Where("task_id = ?", "t-upd").First(&run).Error)
	assert.Equal(t, "v9-updated", run.Message)
}
