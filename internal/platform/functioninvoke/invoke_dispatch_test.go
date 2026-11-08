package functioninvoke

// Invoke 派发面端到端测试：经 Govern → Dispatcher（stub resolver/caller）
// 驱动遥测成功/失败 span、同步空 payload、broadcast 半失败与无活代理、
// 异步任务派发，以及 DeclaredTimeoutMs / ApprovalNotifyRecipients 的
// 早退分支。Govern 闸门细节由 govern_test.go 覆盖，此处只保证派发链贯通。

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// stubCaller 模拟 agent 侧会话：按帧类型回 InvokeResponse / StartTaskResponse。
type stubCaller struct {
	invokePayload []byte
	taskID        string
	err           error
}

func (c *stubCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	if c.err != nil {
		return 0, nil, c.err
	}
	switch msgID {
	case protocol.MsgInvokeRequest:
		req := &sdkv1.InvokeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return 0, nil, err
		}
		resp := &sdkv1.InvokeResponse{Payload: c.invokePayload}
		b, merr := proto.Marshal(resp)
		if merr != nil {
			return 0, nil, merr
		}
		return protocol.MsgInvokeResponse, b, nil
	case protocol.MsgStartTaskRequest:
		resp := &sdkv1.StartTaskResponse{TaskId: c.taskID}
		b, merr := proto.Marshal(resp)
		if merr != nil {
			return 0, nil, merr
		}
		return protocol.MsgStartTaskResponse, b, nil
	}
	return 0, nil, nil
}

type stubResolver struct {
	callers map[string]transport.SessionCaller
}

func (r *stubResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.callers[agentID]
	return c, ok
}

func setupDispatchSvc(t *testing.T, callers map[string]transport.SessionCaller) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db := setupGovernDB(t)
	// :memory: 每个池连接是独立空库，单连接强制同一库（见
	// internal/api/ops 同款修法）。
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	pm, err := policy.NewManager(db, "")
	require.NoError(t, err)
	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)
	if callers != nil {
		dispatcher.SetSessionResolver(&stubResolver{callers: callers})
	}
	tel, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		AdminModel:    model.NewAdminModel(db),
		RoleModel:     model.NewRoleModel(db),
		GameModel:     model.NewGameModel(db),
		RegistryStore: store,
		PolicyManager: pm,
		Dispatcher:    dispatcher,
		Telemetry:     tel,
	}
	return svcCtx, db
}

func dispatchCtx() context.Context {
	ctx := context.WithValue(context.Background(), "username", "alice")
	return svc.WithGameScope(ctx, svc.GameScope{GameID: "test-game", Env: "test-env"})
}

func registerDispatchAgent(t *testing.T, store *reg.Store, agentID string, fnIDs ...string) {
	t.Helper()
	fns := map[string]reg.FunctionMeta{}
	for _, id := range fnIDs {
		fns[id] = reg.FunctionMeta{Enabled: true}
	}
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:   agentID,
		GameID:    "test-game",
		Env:       "test-env",
		Addr:      "127.0.0.1:9100",
		Functions: fns,
		ExpireAt:  time.Now().Add(5 * time.Minute),
		LastSeen:  time.Now(),
	}))
}

func syncReq(id string) *Request {
	return &Request{ID: id, Payload: []byte(`{"x":1}`)}
}

func TestInvoke_SyncSuccessWithTelemetry(t *testing.T) {
	caller := &stubCaller{invokePayload: []byte(`{"echo":true}`)}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")

	resp, err := Invoke(dispatchCtx(), svcCtx, syncReq("demo.echo"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Result)
}

func TestInvoke_SyncDispatchErrorSendsErrorSpan(t *testing.T) {
	caller := &stubCaller{err: context.DeadlineExceeded}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")

	_, err := Invoke(dispatchCtx(), svcCtx, syncReq("demo.echo"))
	require.Error(t, err)
}

func TestInvoke_SyncEmptyPayloadResponse(t *testing.T) {
	caller := &stubCaller{invokePayload: nil}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")

	resp, err := Invoke(dispatchCtx(), svcCtx, syncReq("demo.echo"))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Nil(t, resp.Result, "empty payload must leave Result nil")
}

func TestInvoke_AsyncTaskDispatch(t *testing.T) {
	caller := &stubCaller{taskID: "task-77"}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")

	req := syncReq("demo.echo")
	req.Mode = "async"
	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "task-77", resp.TaskId)
}

func TestInvoke_BroadcastAggregatesResults(t *testing.T) {
	okCaller := &stubCaller{invokePayload: []byte(`{"from":"a1"}`)}
	emptyCaller := &stubCaller{invokePayload: nil}
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{
		"a1": okCaller,
		"a2": emptyCaller,
	})
	seedGovernAdmin(t, db, "alice", "operator")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.echo")
	registerDispatchAgent(t, svcCtx.RegistryStore, "a2", "demo.echo")

	req := syncReq("demo.echo")
	req.Route = "broadcast"
	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Broadcast)
	require.Len(t, resp.Broadcast.Results, 2, "both agent results must be aggregated")
}

func TestInvoke_BroadcastNoLiveAgentRejected(t *testing.T) {
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{})
	seedGovernAdmin(t, db, "alice", "operator")

	req := syncReq("demo.missing")
	req.Route = "broadcast"
	_, err := Invoke(dispatchCtx(), svcCtx, req)
	require.Error(t, err)
}

func TestDeclaredTimeoutMs_EmptyScopeReturnsZero(t *testing.T) {
	svcCtx, _ := setupDispatchSvc(t, nil)
	require.Equal(t, 0, DeclaredTimeoutMs(context.Background(), svcCtx, &Request{}))
	require.Equal(t, 0, DeclaredTimeoutMs(context.Background(), nil, &Request{}))
}

func TestApprovalNotifyRecipients_NilAdminModel(t *testing.T) {
	svcCtx, _ := setupDispatchSvc(t, nil)
	svcCtx.AdminModel = nil
	recipients, err := ApprovalNotifyRecipients(context.Background(), svcCtx)
	require.NoError(t, err)
	require.Nil(t, recipients)
}
