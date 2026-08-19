package function

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
)

// ---------------------------------------------------------------------------
// Fake agent transport (mirrors internal/api/function fixtures)
// ---------------------------------------------------------------------------

type logicFakeSessionCaller struct {
	invokePayload []byte
	startTaskID   string
	err           error
}

func (f *logicFakeSessionCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	switch msgID {
	case protocol.MsgInvokeRequest:
		if f.err != nil {
			return 0, nil, f.err
		}
		resp := &sdkv1.InvokeResponse{Payload: f.invokePayload}
		out, err := proto.Marshal(resp)
		return msgID, out, err
	case protocol.MsgStartTaskRequest:
		if f.err != nil {
			return 0, nil, f.err
		}
		resp := &sdkv1.StartTaskResponse{TaskId: f.startTaskID}
		out, err := proto.Marshal(resp)
		return msgID, out, err
	default:
		return 0, nil, assert.AnError
	}
}

type logicFakeResolver struct {
	callers map[string]transport.SessionCaller
}

func (r *logicFakeResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.callers[agentID]
	return c, ok
}

type invokeLogicFixture struct {
	svcCtx  *svc.ServiceContext
	store   *reg.Store
	caller  *logicFakeSessionCaller
	ctx     context.Context
	ctxUser context.Context
	game    *model.Game
}

func newInvokeLogicFixture(t *testing.T) *invokeLogicFixture {
	t.Helper()
	svcCtx, _ := setupFullTestContext(t)
	db := svcCtx.DB

	store := reg.NewStore()
	resolver := &logicFakeResolver{callers: map[string]transport.SessionCaller{}}
	dispatcher := dispatch.NewDispatcher(store)
	dispatcher.SetSessionResolver(resolver)

	policyManager, err := policymgr.NewManager(db, "")
	require.NoError(t, err)

	caller := &logicFakeSessionCaller{}
	resolver.callers["agent-1"] = caller
	resolver.callers["agent-2"] = caller

	svcCtx.PolicyManager = policyManager
	svcCtx.Dispatcher = dispatcher

	f := &invokeLogicFixture{
		svcCtx: svcCtx,
		store:  store,
		caller: caller,
	}
	f.ctx = context.Background()
	f.ctxUser = context.WithValue(context.Background(), "username", "testadmin")

	seedInvokeAgent(t, store)
	return f
}

func seedInvokeAgent(t *testing.T, store *reg.Store) {
	t.Helper()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "demo",
		Env:       "prod",
		Addr:      "127.0.0.1:9001",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Enabled: true}},
	}))
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-2",
		GameID:    "demo",
		Env:       "prod",
		Addr:      "127.0.0.1:9002",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"player.ban": {Enabled: true}},
	}))
}

// ---------------------------------------------------------------------------
// FunctionInvoke: success and dispatch error paths
// ---------------------------------------------------------------------------

func TestFunctionInvoke_SyncSuccess(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.invokePayload = []byte(`{"banned":true}`)

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:      "player.ban",
		Payload: []byte(`{"playerId":"p1"}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"banned":true}`, string(resp.Result))
}

func TestFunctionInvoke_SyncDispatchError(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.err = assert.AnError

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{ID: "player.ban"})
	require.Error(t, err)
}

func TestFunctionInvoke_TaskSuccess(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.startTaskID = "task-123"

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Mode: "task",
	})
	require.NoError(t, err)
	assert.Equal(t, "task-123", resp.TaskID)
	assert.Equal(t, "task-123", resp.TaskId)
}

func TestFunctionInvoke_TaskDispatchError(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.err = assert.AnError

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Mode: "start_task",
	})
	require.Error(t, err)
}

func TestFunctionInvoke_BroadcastSuccess(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.invokePayload = []byte(`{"ok":1}`)

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "broadcast",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Broadcast)
	assert.Equal(t, 2, resp.Broadcast.Total)
	assert.Equal(t, 2, resp.Broadcast.Success)
	assert.Equal(t, 0, resp.Broadcast.Failure)
	assert.Len(t, resp.Broadcast.Results, 2)
	assert.JSONEq(t, `{"ok":1}`, string(resp.Result))
}

func TestFunctionInvoke_BroadcastPartialFailure(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.err = assert.AnError

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	// Per-agent failures are aggregated, not surfaced as a top-level error.
	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "broadcast",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Broadcast)
	assert.Equal(t, 2, resp.Broadcast.Failure)
	assert.Equal(t, 0, resp.Broadcast.Success)
}

func TestFunctionInvoke_AdminLoadFails(t *testing.T) {
	f := newInvokeLogicFixture(t)
	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "ghost"), f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{ID: "player.ban"})
	require.Error(t, err)
}

func TestFunctionInvoke_RouteValidation(t *testing.T) {
	f := newInvokeLogicFixture(t)
	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)

	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "targeted",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target_service_id is required")

	_, err = logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "hash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash_key is required")

	_, err = logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "broadcast", Mode: "task",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for synchronous")

	_, err = logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "targeted", TargetServiceID: "svc-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no live agent")

	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "hash", HashKey: "player-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// setupNonAdminScope wires GameModel + PermissionService + a game row so
// non-admin invoke requests pass RequireGameEnvScope and reach the
// function-level permission check.
func (f *invokeLogicFixture) setupNonAdminScope(t *testing.T) {
	t.Helper()
	f.svcCtx.GameModel = model.NewGameModel(f.svcCtx.DB)
	f.svcCtx.PermissionService = permission.NewPermissionService(f.svcCtx.DB)
	game := &model.Game{GameID: "demo", Name: "Demo Game"}
	require.NoError(t, f.svcCtx.DB.Create(game).Error)
	f.game = game
}

// grantGameScope gives the named non-admin admin access to the seeded game.
func (f *invokeLogicFixture) grantGameScope(t *testing.T, username string) {
	t.Helper()
	admin, err := f.svcCtx.AdminModel.FindByUsername(context.Background(), username)
	require.NoError(t, err)
	require.NoError(t, f.svcCtx.AdminModel.SetGameScope(context.Background(), admin.ID, f.game.ID))
}

func TestFunctionInvoke_NonAdminPermissionDenied(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.setupNonAdminScope(t)
	seedNonAdminUser(t, f.svcCtx, "operator", "function:none")

	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "operator"), f.svcCtx)
	f.grantGameScope(t, "operator")
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", GameID: "demo", Env: "prod",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestFunctionInvoke_NonAdminMissingGameScope(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.setupNonAdminScope(t)
	seedNonAdminUser(t, f.svcCtx, "operator3", "function:invoke")

	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "operator3"), f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{ID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game_id is required")
}

func TestFunctionInvoke_NonAdminWithInvokePermission(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.setupNonAdminScope(t)
	f.caller.invokePayload = []byte(`{"ok":true}`)
	seedNonAdminUser(t, f.svcCtx, "operator2", "function:invoke")

	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "operator2"), f.svcCtx)
	f.grantGameScope(t, "operator2")
	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", GameID: "demo", Env: "prod",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func seedNonAdminUser(t *testing.T, svcCtx *svc.ServiceContext, username, perm string) {
	t.Helper()
	admin := &model.Admin{Username: username, Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), admin, "password"))
	role := &model.Role{Name: username + "-role", Description: "role"}
	require.NoError(t, svcCtx.RoleModel.Create(context.Background(), role))
	require.NoError(t, svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID))
	require.NoError(t, svcCtx.RoleModel.ReplacePermissions(context.Background(), role.ID, []string{perm}))
}

// ---------------------------------------------------------------------------
// buildBroadcastResponse with successes and failures
// ---------------------------------------------------------------------------

func TestBuildBroadcastResponse_MixedResults(t *testing.T) {
	b := &dispatch.BroadcastInvocation{
		Total: 3,
		Successes: []*dispatch.BroadcastAgentResult{
			{AgentID: "a1", Response: &sdkv1.InvokeResponse{Payload: []byte(`{"n":1}`)}},
			{AgentID: "a2", Response: &sdkv1.InvokeResponse{Payload: []byte(`{"n":2}`)}},
			{AgentID: "a3"},
		},
		Failures: []*dispatch.BroadcastAgentResult{
			{AgentID: "a4", Err: assert.AnError},
		},
	}
	out := buildBroadcastResponse(b)
	require.NotNil(t, out.Broadcast)
	assert.Equal(t, 3, out.Broadcast.Total)
	assert.Equal(t, 3, out.Broadcast.Success)
	assert.Equal(t, 1, out.Broadcast.Failure)
	assert.Len(t, out.Broadcast.Results, 4)
	assert.JSONEq(t, `{"n":1}`, string(out.Result))
	assert.Equal(t, "a4", out.Broadcast.Results[3].AgentID)
	assert.NotEmpty(t, out.Broadcast.Results[3].Error)
}

// ---------------------------------------------------------------------------
// Batch operations
// ---------------------------------------------------------------------------

func seedFunctionForBatch(t *testing.T, svcCtx *svc.ServiceContext, functionID string) {
	t.Helper()
	require.NoError(t, svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: functionID,
		Name:       functionID,
		Status:     model.StatusEnabled,
	}))
}

func TestBatchUpdateFunctions_AllBranches(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "batch.update.one")

	logic := NewBatchUpdateFunctionsLogic(ctx, svcCtx)

	resp, err := logic.BatchUpdateFunctions(&BatchUpdateFunctionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Empty(t, resp.Failed)

	resp, err = logic.BatchUpdateFunctions(&BatchUpdateFunctionsRequest{
		FunctionIds: []string{"batch.update.one", "missing.fn"}, Enabled: false,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Updated)

	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "batch.update.one")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDisabled, fn.Status)
}

func TestBatchDeleteFunctions_AllBranches(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "batch.delete.one")

	logic := NewBatchDeleteFunctionsLogic(ctx, svcCtx)

	resp, err := logic.BatchDeleteFunctions(&BatchDeleteFunctionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)

	resp, err = logic.BatchDeleteFunctions(&BatchDeleteFunctionsRequest{
		FunctionIds: []string{"batch.delete.one", "missing.fn"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Updated)

	_, err = svcCtx.FunctionModel.FindByFunctionID(ctx, "batch.delete.one")
	require.Error(t, err)
}

func TestBatchCopyFunctions_AllBranches(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "batch.copy.one")

	logic := NewBatchCopyFunctionsLogic(ctx, svcCtx)

	resp, err := logic.BatchCopyFunctions(&BatchCopyFunctionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Empty(t, resp.Failed)
	assert.Empty(t, resp.Copied)

	resp, err = logic.BatchCopyFunctions(&BatchCopyFunctionsRequest{
		FunctionIds: []string{"batch.copy.one", "missing.fn"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Updated)
	assert.Len(t, resp.Copied, 1)
	assert.Contains(t, resp.Failed, "missing.fn")
}

// ---------------------------------------------------------------------------
// FunctionAnalytics with ConfigVersionModel
// ---------------------------------------------------------------------------

func TestFunctionAnalytics_WithConfigVersions(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.ConfigVersionModel = model.NewConfigVersionModel(svcCtx.DB)

	_, errCreate := svcCtx.ConfigVersionModel.Create(ctx, "function_form:analytics.fn", `{"x":1}`, "tester")
	require.NoError(t, errCreate)
	_, errCreate = svcCtx.ConfigVersionModel.Create(ctx, "function_form:analytics.fn", `{"x":2}`, "tester")
	require.NoError(t, errCreate)

	logic := NewFunctionAnalyticsLogic(ctx, svcCtx)
	resp, err := logic.FunctionAnalytics(&FunctionAnalyticsRequest{ID: "analytics.fn"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.TotalCalls)
	assert.Equal(t, int64(2), resp.CallsToday)
	assert.Equal(t, int64(2), resp.CallsThisWeek)
	assert.Equal(t, int64(2), resp.CallsThisMonth)
	assert.Equal(t, float64(100), resp.SuccessRate)
}

func TestFunctionAnalytics_InvalidID(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionAnalyticsLogic(ctx, svcCtx)
	_, err := logic.FunctionAnalytics(&FunctionAnalyticsRequest{ID: "   "})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionHistory pagination + FunctionHistoryPaged
// ---------------------------------------------------------------------------

func TestFunctionHistory_Pagination(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.ConfigVersionModel = model.NewConfigVersionModel(svcCtx.DB)
	_, errCreate := svcCtx.ConfigVersionModel.Create(ctx, "function_form:history.fn", `{"a":1}`, "tester")
	require.NoError(t, errCreate)
	_, errCreate = svcCtx.ConfigVersionModel.Create(ctx, "function_form:history.fn", `{"a":2}`, "tester")
	require.NoError(t, errCreate)

	logic := NewFunctionHistoryLogic(ctx, svcCtx)

	items, total, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "history.fn"})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)

	items, total, err = logic.FunctionHistory(&FunctionHistoryRequest{ID: "history.fn", Offset: 100})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Empty(t, items)

	items, _, err = logic.FunctionHistory(&FunctionHistoryRequest{ID: "history.fn", Limit: 1})
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, _, err = logic.FunctionHistoryPaged(&FunctionHistoryRequest{ID: "history.fn", Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestFunctionHistory_ConfigVersionListError(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.ConfigVersionModel = model.NewConfigVersionModel(svcCtx.DB)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("config_versions"))

	logic := NewFunctionHistoryLogic(ctx, svcCtx)
	_, _, err := logic.FunctionHistory(&FunctionHistoryRequest{ID: "history.err"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionEnable / FunctionDisable success paths
// ---------------------------------------------------------------------------

func TestFunctionDisableAndEnable_Success(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "toggle.fn")

	disable := NewFunctionDisableLogic(ctx, svcCtx)
	require.NoError(t, disable.FunctionDisable(&FunctionActionRequest{ID: "toggle.fn"}))
	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, "toggle.fn")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDisabled, fn.Status)

	enable := NewFunctionEnableLogic(ctx, svcCtx)
	require.NoError(t, enable.FunctionEnable(&FunctionActionRequest{ID: "toggle.fn"}))
	fn, err = svcCtx.FunctionModel.FindByFunctionID(ctx, "toggle.fn")
	require.NoError(t, err)
	assert.Equal(t, model.StatusEnabled, fn.Status)
}

func TestFunctionDisable_NotFound(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionDisableLogic(ctx, svcCtx)
	err := logic.FunctionDisable(&FunctionActionRequest{ID: "ghost.fn"})
	require.Error(t, err)
}

func TestFunctionEnable_NotFound(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	logic := NewFunctionEnableLogic(ctx, svcCtx)
	err := logic.FunctionEnable(&FunctionActionRequest{ID: "ghost.fn"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionPermissions / Update permission branches
// ---------------------------------------------------------------------------

func TestFunctionPermissions_NonAdminForbiddenAndAllowed(t *testing.T) {
	svcCtx, _ := setupFullTestContext(t)

	seedNonAdminUser(t, svcCtx, "viewer", "none:read")
	logic := NewFunctionPermissionsLogic(context.WithValue(context.Background(), "username", "viewer"), svcCtx)
	_, err := logic.FunctionPermissions(&FunctionPermissionsRequest{ID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看函数权限")

	seedNonAdminUser(t, svcCtx, "permwriter", "permission:write")
	logic = NewFunctionPermissionsLogic(context.WithValue(context.Background(), "username", "permwriter"), svcCtx)
	resp, err := logic.FunctionPermissions(&FunctionPermissionsRequest{ID: "player.ban"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionPermissionsUpdate_FullFlow(t *testing.T) {
	svcCtx, _ := setupFullTestContext(t)
	seedNonAdminUser(t, svcCtx, "permadmin", "roles:manage")

	logic := NewFunctionPermissionsUpdateLogic(context.WithValue(context.Background(), "username", "permadmin"), svcCtx)
	resp, err := logic.FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID: "player.ban",
		Permissions: []FunctionPermission{{
			Resource: "player",
			Actions:  []string{"invoke"},
			Roles:    []string{"operator"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].Resource)
}

func TestFunctionPermissionsUpdate_NonAdminForbidden(t *testing.T) {
	svcCtx, _ := setupFullTestContext(t)
	seedNonAdminUser(t, svcCtx, "noperm", "none:write")

	logic := NewFunctionPermissionsUpdateLogic(context.WithValue(context.Background(), "username", "noperm"), svcCtx)
	_, err := logic.FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{ID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权更新函数权限")
}

// ---------------------------------------------------------------------------
// DescriptorsV2 checkReadPermission branches
// ---------------------------------------------------------------------------

func TestDescriptorsV2_PermissionBranches(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)

	// Anonymous (no username) skips permission checks.
	logic := NewDescriptorsLogic(ctx, svcCtx)
	_, err := logic.DescriptorsV2(&DescriptorsRequest{})
	require.Error(t, err) // scope error comes next
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	// Non-admin without functions:read is forbidden.
	seedNonAdminUser(t, svcCtx, "catalogviewer", "none:read")
	logic = NewDescriptorsLogic(context.WithValue(ctx, "username", "catalogviewer"), svcCtx)
	_, err = logic.DescriptorsV2(&DescriptorsRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问函数目录")

	// Non-admin with functions:read passes the permission check.
	seedNonAdminUser(t, svcCtx, "catalogreader", "functions:read")
	logic = NewDescriptorsLogic(context.WithValue(ctx, "username", "catalogreader"), svcCtx)
	_, err = logic.DescriptorsV2(&DescriptorsRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

// ---------------------------------------------------------------------------
// getOrCreateFunctionRecordWithRisk branches
// ---------------------------------------------------------------------------

func TestGetOrCreateFunctionRecordWithRisk_Guards(t *testing.T) {
	_, err := getOrCreateFunctionRecordWithRisk(context.Background(), nil, "fn", "")
	require.Error(t, err)

	svcCtx := &svc.ServiceContext{}
	_, err = getOrCreateFunctionRecordWithRisk(context.Background(), svcCtx, "fn", "")
	require.Error(t, err)
}

func TestGetOrCreateFunctionRecordWithRisk_PolicyManager(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	policyManager, err := policymgr.NewManager(svcCtx.DB, "")
	require.NoError(t, err)
	svcCtx.PolicyManager = policyManager

	fn, err := getOrCreateFunctionRecordWithRisk(ctx, svcCtx, "policy.fn", "high")
	require.NoError(t, err)
	assert.Equal(t, "policy.fn", fn.FunctionID)
}

// ---------------------------------------------------------------------------
// FunctionsPending with data
// ---------------------------------------------------------------------------

func TestFunctionsPending_WithData(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Create(&model.PendingFunction{
		FunctionID:  "pending.fn",
		Payload:     datatypes.JSONMap{"name": "Pending Fn"},
		RequestedBy: "agent-1",
		Status:      "pending",
	}).Error)

	logic := NewFunctionsPendingLogic(ctx, svcCtx)
	resp, err := logic.FunctionsPending(&FunctionsPendingRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "pending.fn", resp.Items[0].ID)
	assert.Equal(t, "Pending Fn", resp.Items[0].Name)
	assert.Equal(t, "agent-1", resp.Items[0].Requester)
}
