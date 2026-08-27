package function

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- fake agent session transport ---

type fakeSessionCaller struct {
	invokePayload []byte
	startTaskID   string
	err           error
	requests      []*sdkv1.InvokeRequest
}

func (f *fakeSessionCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	switch msgID {
	case protocol.MsgInvokeRequest:
		req := &sdkv1.InvokeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return 0, nil, err
		}
		f.requests = append(f.requests, req)
		if f.err != nil {
			return 0, nil, f.err
		}
		resp := &sdkv1.InvokeResponse{Payload: f.invokePayload}
		out, err := proto.Marshal(resp)
		return msgID, out, err
	case protocol.MsgStartTaskRequest:
		req := &sdkv1.InvokeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return 0, nil, err
		}
		f.requests = append(f.requests, req)
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

type fakeResolver struct {
	callers map[string]transport.SessionCaller
}

func (r *fakeResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.callers[agentID]
	return c, ok
}

// --- test fixture ---

type invokeFixture struct {
	db        *gorm.DB
	svcCtx    *svc.ServiceContext
	store     *reg.Store
	resolver  *fakeResolver
	auditLog  *audit.AuditService
	approvals *approvals.MemStore
	caller    *fakeSessionCaller
}

func newInvokeFixture(t *testing.T) *invokeFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	store := reg.NewStore()
	resolver := &fakeResolver{callers: map[string]transport.SessionCaller{}}
	dispatcher := dispatch.NewDispatcher(store)
	dispatcher.SetSessionResolver(resolver)

	policyManager, err := policymgr.NewManager(db, "")
	require.NoError(t, err)

	approvalsStore := approvals.NewMemStore()
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)

	f := &invokeFixture{
		db:        db,
		store:     store,
		resolver:  resolver,
		auditLog:  auditSvc,
		approvals: approvalsStore,
	}
	f.svcCtx = &svc.ServiceContext{
		DB:             db,
		AdminModel:     model.NewAdminModel(db),
		RoleModel:      model.NewRoleModel(db),
		FunctionModel:  model.NewFunctionModel(db),
		RegistryStore:  store,
		PolicyManager:  policyManager,
		ApprovalsStore: approvalsStore,
		AuditService:   auditSvc,
		Dispatcher:     dispatcher,
		Telemetry:      nil,
	}
	return f
}

func (f *invokeFixture) registerAgent(t *testing.T, agentID string, fnIDs ...string) {
	t.Helper()
	fns := map[string]reg.FunctionMeta{}
	for _, id := range fnIDs {
		fns[id] = reg.FunctionMeta{Enabled: true}
	}
	require.NoError(t, f.store.UpsertAgent(&reg.AgentSession{
		AgentID:   agentID,
		GameID:    "demo",
		Env:       "prod",
		Addr:      "127.0.0.1:9000",
		Functions: fns,
		ExpireAt:  time.Now().Add(5 * time.Minute),
		LastSeen:  time.Now(),
	}))
}

func (f *invokeFixture) createOperator(t *testing.T, username, roleName string) {
	t.Helper()
	ctx := context.Background()
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, f.svcCtx.AdminModel.Create(ctx, admin, "pw"))
	if roleName != "" {
		role := &model.Role{Name: roleName, Category: "test"}
		require.NoError(t, f.svcCtx.RoleModel.Create(ctx, role))
		require.NoError(t, f.svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))
	}
}

func (f *invokeFixture) ctxFor(username string) context.Context {
	ctx := context.WithValue(context.Background(), "username", username)
	return svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
}

// --- functionInvoke scenarios ---

func TestFunctionInvoke_SyncSuccess(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller

	svcAPI := NewService(f.svcCtx)
	ctx := f.ctxFor("opuser")

	resp, err := svcAPI.FunctionInvoke(ctx, &FunctionInvokeRequest{ID: "demo.echo", Payload: []byte(`{"x":1}`)})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.JSONEq(t, `{"echo":true}`, string(resp.Result))
	assert.Equal(t, "demo", resp.ExecutionMetadata["game_id"])
	assert.Equal(t, "prod", resp.ExecutionMetadata["env"])
	assert.Equal(t, "opuser", resp.ExecutionMetadata["actor"])
	assert.Equal(t, "false", resp.ExecutionMetadata["async"])

	require.Len(t, f.caller.requests, 1)
	sent := f.caller.requests[0]
	assert.Equal(t, "demo.echo", sent.GetFunctionId())
	assert.Equal(t, "agent-1", sent.GetMetadata()["agent_id"])
	assert.JSONEq(t, `{"x":1}`, string(sent.GetPayload()))
}

func TestFunctionInvoke_AsyncTask(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.job")
	f.caller = &fakeSessionCaller{startTaskID: "task-42"}
	f.resolver.callers["agent-1"] = f.caller

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.job", Mode: "async", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "task-42", resp.TaskId)
	assert.Equal(t, "task-42", resp.TaskID)
	assert.Equal(t, "true", resp.ExecutionMetadata["async"])
}

func TestFunctionInvoke_BroadcastMixedOutcomes(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-a", "demo.fanout")
	f.registerAgent(t, "agent-b", "demo.fanout")

	ok := &fakeSessionCaller{invokePayload: []byte(`{"from":"a"}`)}
	bad := &fakeSessionCaller{err: assert.AnError}
	f.resolver.callers["agent-a"] = ok
	f.resolver.callers["agent-b"] = bad

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.fanout", Route: "broadcast",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Broadcast)
	assert.Equal(t, 2, resp.Broadcast.Total)
	assert.Equal(t, 1, resp.Broadcast.Success)
	assert.Equal(t, 1, resp.Broadcast.Failure)
	require.Len(t, resp.Broadcast.Results, 2)

	byAgent := map[string]BroadcastAgentItem{}
	for _, item := range resp.Broadcast.Results {
		byAgent[item.AgentID] = item
	}
	assert.JSONEq(t, `{"from":"a"}`, string(byAgent["agent-a"].Result))
	assert.Empty(t, byAgent["agent-b"].Error == "", "failed agent must carry an error")
	assert.Contains(t, byAgent["agent-b"].Error, "assert.AnError")

	// Legacy Result keeps the first successful payload.
	assert.JSONEq(t, `{"from":"a"}`, string(resp.Result))
}

func TestFunctionInvoke_ApprovalFlow(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")

	// High-risk override forces approval; the invoke must never reach the agent.
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", &policymgr.Policy{
		FunctionID:       "danger.blast",
		AllowedRoles:     []string{},
		RequireApproval:  true,
		RequireAudit:     true,
		ApprovalWorkflow: "two_person",
		DefaultRiskLevel: "high",
	}))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "danger.blast", Payload: []byte(`{"n":1}`),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.ApprovalRequired)
	assert.Equal(t, "two_person", resp.ApprovalWorkflow)
	assert.NotEmpty(t, resp.ApprovalID)
	assert.Nil(t, resp.Result)

	stored, err := f.approvals.Get(resp.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "pending", stored.State)
	assert.Equal(t, "danger.blast", stored.FunctionID)
	assert.Equal(t, "demo", stored.GameID)
	assert.Equal(t, "prod", stored.Env)
	assert.Equal(t, "opuser", stored.Actor)
	assert.Equal(t, "lb", stored.Route)
	assert.JSONEq(t, `{"n":1}`, string(stored.Payload))

	// Approval creation is audited when the policy requires audit.
	records, total, err := f.auditLog.List(audit.AuditFilter{}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	assert.NotEmpty(t, records)
}

func TestFunctionInvoke_ApprovalBypassContinuation(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", &policymgr.Policy{
		FunctionID:      "danger.blast",
		RequireApproval: true,
	}))

	f.caller = &fakeSessionCaller{invokePayload: []byte(`"done"`)}
	f.resolver.callers["agent-1"] = f.caller

	req := &FunctionInvokeRequest{
		ID:       "danger.blast",
		Payload:  []byte(`{}`),
		Metadata: map[string]string{"approval_bypass": "approved"},
	}
	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), req)
	require.NoError(t, err)
	assert.False(t, resp.ApprovalRequired)
	assert.JSONEq(t, `"done"`, string(resp.Result))
}

func TestFunctionInvoke_PageSnapshotGovernedSkipsPolicy(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "gov.fn")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "gov.fn", &policymgr.Policy{
		FunctionID:   "gov.fn",
		AllowedRoles: []string{"nobody"},
	}))

	// The snapshot-governed flag must bypass the role restriction.
	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID:       "gov.fn",
		Metadata: map[string]string{"page_snapshot_governance": "validated"},
	})
	// No agent connection is registered: policy is skipped and dispatch fails.
	require.Error(t, err)
	require.NotNil(t, resp)
}

func TestFunctionInvoke_RoleDenied(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "operator")
	f.registerAgent(t, "agent-1", "locked.fn")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "locked.fn", &policymgr.Policy{
		FunctionID:   "locked.fn",
		AllowedRoles: []string{"admin"},
	}))

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{ID: "locked.fn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestFunctionInvoke_DispatchErrorReturnsResponseAndError(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	// No agents registered for the function.

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{ID: "ghost.fn"})
	require.Error(t, err)
	require.NotNil(t, resp, "error invocations still carry a traceable response")
	assert.Nil(t, resp.Result)
	assert.Empty(t, resp.TaskId)
}

func TestFunctionInvoke_ValidationAndScopeErrors(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	svcAPI := NewService(f.svcCtx)

	_, err := svcAPI.FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{Route: "targeted"})
	require.Error(t, err)

	_, err = svcAPI.FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{Route: "hash"})
	require.Error(t, err)

	_, err = svcAPI.FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{Route: "bogus"})
	require.Error(t, err)

	// Missing scope.
	_, err = svcAPI.FunctionInvoke(context.WithValue(context.Background(), "username", "opuser"), &FunctionInvokeRequest{ID: "demo.fn"})
	require.Error(t, err)

	// Missing admin (no username in context).
	_, err = svcAPI.FunctionInvoke(f.ctxFor(""), &FunctionInvokeRequest{ID: "demo.fn"})
	require.Error(t, err)
}

func TestFunctionInvoke_TargetedRouting(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-a", "svc.fn")
	f.registerAgent(t, "agent-b", "svc.fn")
	// Only agent-b owns provider service "go-sdk-1".
	require.NoError(t, f.store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-b",
		GameID:    "demo",
		Env:       "prod",
		Functions: map[string]reg.FunctionMeta{"svc.fn": {Enabled: true}},
		Providers: []reg.ProviderSession{{
			ProviderID:  "go-sdk-1",
			GameID:      "demo",
			FunctionIDs: []string{"svc.fn"},
			Addr:        "127.0.0.1:9100",
		}},
		ExpireAt: time.Now().Add(5 * time.Minute),
	}))

	targeted := &fakeSessionCaller{invokePayload: []byte(`"b"`)}
	f.resolver.callers["agent-b"] = targeted

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "svc.fn", Route: "targeted", TargetServiceID: "go-sdk-1",
	})
	require.NoError(t, err)
	assert.JSONEq(t, `"b"`, string(resp.Result))
	assert.Equal(t, "agent-b", targeted.requests[0].GetMetadata()["agent_id"])
}

// --- buildBroadcastResponse unit test ---

func TestBuildBroadcastResponse(t *testing.T) {
	t.Run("nil invocation", func(t *testing.T) {
		resp := buildBroadcastResponse(nil)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Broadcast)
		assert.Equal(t, 0, resp.Broadcast.Total)
	})

	t.Run("mixed outcomes", func(t *testing.T) {
		b := &dispatch.BroadcastInvocation{
			Total: 3,
			Successes: []*dispatch.BroadcastAgentResult{
				{AgentID: "a1", Response: &sdkv1.InvokeResponse{Payload: []byte(`{"r":1}`)}},
				{AgentID: "a2", Response: &sdkv1.InvokeResponse{}},
			},
			Failures: []*dispatch.BroadcastAgentResult{
				{AgentID: "a3", Err: assert.AnError},
			},
		}
		resp := buildBroadcastResponse(b)
		assert.Equal(t, 3, resp.Broadcast.Total)
		assert.Equal(t, 2, resp.Broadcast.Success)
		assert.Equal(t, 1, resp.Broadcast.Failure)
		require.Len(t, resp.Broadcast.Results, 3)
		assert.JSONEq(t, `{"r":1}`, string(resp.Broadcast.Results[0].Result))
		assert.Nil(t, resp.Broadcast.Results[1].Result)
		assert.Contains(t, resp.Broadcast.Results[2].Error, "assert.AnError")
		// Legacy Result mirrors the first successful payload only.
		assert.JSONEq(t, `{"r":1}`, string(resp.Result))
	})
}

// --- enable/disable flip the addressed function ---

func TestFunctionEnableDisable_TogglesStatus(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.WithValue(context.Background(), "username", "opuser")
	fn := &model.Function{FunctionID: "toggle.me", Name: "Toggle", Status: 1}
	require.NoError(t, f.db.WithContext(ctx).Create(fn).Error)
	other := &model.Function{FunctionID: "keep.me", Name: "Keep", Status: 1}
	require.NoError(t, f.db.WithContext(ctx).Create(other).Error)

	// 修复回归：此前硬编码主键 0 导致 WHERE 不命中、静默无效。
	require.NoError(t, functionDisable(ctx, f.svcCtx, &FunctionDisableRequest{FunctionId: "toggle.me"}))
	var after model.Function
	require.NoError(t, f.db.WithContext(ctx).Where("function_id = ?", "toggle.me").First(&after).Error)
	assert.Equal(t, 0, after.Status, "disable must flip status")

	// 其他函数不受影响
	var untouched model.Function
	require.NoError(t, f.db.WithContext(ctx).Where("function_id = ?", "keep.me").First(&untouched).Error)
	assert.Equal(t, 1, untouched.Status)

	require.NoError(t, functionEnable(ctx, f.svcCtx, &FunctionEnableRequest{FunctionId: "toggle.me"}))
	require.NoError(t, f.db.WithContext(ctx).Where("function_id = ?", "toggle.me").First(&after).Error)
	assert.Equal(t, 1, after.Status, "enable must restore status")

	// 空 functionId 应显式报错而非静默成功
	require.Error(t, functionDisable(ctx, f.svcCtx, &FunctionDisableRequest{FunctionId: "  "}))
	// 不存在的函数应报错
	require.Error(t, functionDisable(ctx, f.svcCtx, &FunctionDisableRequest{FunctionId: "missing.fn"}))
}

// --- instances listing with provider sessions ---

func TestFunctionInstancesAll_ProviderClaimsAndFallback(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-p",
		GameID:  "demo",
		Env:     "prod",
		Functions: map[string]reg.FunctionMeta{
			"claimed.fn":   {Enabled: true},
			"unclaimed.fn": {Enabled: true},
		},
		Providers: []reg.ProviderSession{{
			ProviderID:  "js-sdk",
			GameID:      "demo",
			Addr:        "127.0.0.1:9200",
			Version:     "v2",
			SDKLanguage: "javascript",
			SDKName:     "croupier-js-sdk",
			SDKVersion:  "1.4.0",
			FunctionIDs: []string{"claimed.fn", "claimed.fn"},
		}},
		ExpireAt: time.Now().Add(5 * time.Minute),
		LastSeen: time.Now(),
	}))

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(f.ctxFor("opuser"), &FunctionInstancesAllRequest{})
	require.NoError(t, err)

	byKey := map[string]FunctionInstanceSummary{}
	for _, item := range resp.Instances {
		byKey[item.FunctionID+"/"+item.ServiceID] = item
	}

	claimed := byKey["claimed.fn/js-sdk"]
	assert.Equal(t, "agent-p", claimed.AgentID)
	assert.Equal(t, "js-sdk", claimed.ServiceID)
	assert.Equal(t, "javascript", claimed.SDKLang)
	assert.Equal(t, "v2", claimed.Version)

	fallback := byKey["unclaimed.fn/"]
	require.NotNil(t, fallback, "unclaimed functions must fall back to agent-level rows")
	assert.Equal(t, "agent-p", fallback.AgentID)

	// Duplicated provider function IDs collapse to one row.
	count := 0
	for key := range byKey {
		if key == "claimed.fn/js-sdk" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestFunctionInstances_NilRegistryStore(t *testing.T) {
	svcAPI := NewService(&svc.ServiceContext{})
	resp, err := svcAPI.FunctionInstances(context.Background(), &FunctionInstancesRequest{ID: "fn"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	respAll, err := svcAPI.FunctionInstancesAll(context.Background(), &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	assert.Empty(t, respAll.Instances)
}

// --- DB error branches ---

func TestFunctionsList_DBError(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	require.NoError(t, f.db.Migrator().DropTable("functions"))

	_, err := NewService(f.svcCtx).FunctionsList(f.ctxFor("opuser"), &FunctionsListRequest{})
	require.Error(t, err)
}

func TestDescriptors_DBError(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("function_descriptors"))

	_, err := NewService(f.svcCtx).Descriptors(f.ctxFor("opuser"), &DescriptorsRequest{})
	require.Error(t, err)
}

func TestFunctionPermissions_ListAndReplace(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("opuser")

	// Non-array / non-string payloads fall back to empty lists.
	require.NoError(t, f.db.WithContext(ctx).Create(&model.FunctionPermission{
		FunctionID: "perm.fn",
		Resource:   "function",
		Roles:      model.JSON(`["viewer"]`),
		Actions:    model.JSON(`["invoke"]`),
	}).Error)

	resp, err := NewService(f.svcCtx).FunctionPermissions(ctx, &FunctionPermissionsRequest{ID: "perm.fn"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, []string{"viewer"}, resp.Items[0].Roles)
	assert.Equal(t, []string{"invoke"}, resp.Items[0].Actions)

	require.NoError(t, NewService(f.svcCtx).FunctionPermissionsUpdate(ctx, &FunctionPermissionsUpdateRequest{
		ID: "perm.fn",
		Permissions: []FunctionPermission{
			{Resource: "function", Roles: []string{"admin"}, Actions: []string{"invoke", "read"}},
		},
	}))

	resp, err = NewService(f.svcCtx).FunctionPermissions(ctx, &FunctionPermissionsRequest{ID: "perm.fn"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, []string{"admin"}, resp.Items[0].Roles)
	assert.Len(t, resp.Items[0].Actions, 2)
}

func TestFunctionDelete_MissingFunction(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.WithValue(context.Background(), "username", "opuser")
	// Deleting a non-existent function id succeeds silently (no rows matched).
	err := NewService(f.svcCtx).FunctionDelete(ctx, &FunctionDeleteRequest{FunctionId: "ghost"})
	assert.NoError(t, err)
}

func TestBatchDeleteFunctions_MixedResults(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.WithValue(context.Background(), "username", "opuser")
	fn := &model.Function{FunctionID: "batch.fn", Name: "Batch", Status: 1}
	require.NoError(t, f.db.WithContext(ctx).Create(fn).Error)

	resp, err := NewService(f.svcCtx).BatchDeleteFunctions(ctx, &BatchDeleteFunctionsRequest{
		FunctionIds: []string{"batch.fn", "missing.fn"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"batch.fn", "missing.fn"}, resp.Deleted)
	assert.Empty(t, resp.Failed)
}

func TestFunctionHistory_LimitClamping(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	ctx := f.ctxFor("opuser")

	// Negative offset and oversized limit are clamped without error.
	resp, err := NewService(f.svcCtx).FunctionHistory(ctx, &FunctionHistoryRequest{
		ID: "hist.fn", Limit: 999999, Offset: -5,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Items)
	assert.GreaterOrEqual(t, resp.Total, 1) // policy-manager default creation writes one history row
}

func TestFunctionWarnings_WithRegistry(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("opuser")

	resp, err := NewService(f.svcCtx).FunctionWarnings(ctx, &FunctionWarningsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	resp, err = NewService(&svc.ServiceContext{}).FunctionWarnings(ctx, &FunctionWarningsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}
