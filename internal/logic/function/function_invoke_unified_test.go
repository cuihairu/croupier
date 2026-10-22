package function_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	functionapi "github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 统一收口语义等价回归：internal/api/function（生产 HTTP 入口）与
// internal/logic/function（logic 薄壳）在同一个 svcCtx 上必须表现一致——
// 审批、禁用、分配与执行留痕四面均由 internal/platform/functioninvoke
// 的唯一执行链承担，两侧不允许再出现旁路语义。

type unifiedFakeCaller struct {
	invokePayload []byte
	startTaskID   string
	err           error
	requests      []*sdkv1.InvokeRequest
}

func (f *unifiedFakeCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
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
		out, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: f.invokePayload})
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
		out, err := proto.Marshal(&sdkv1.StartTaskResponse{TaskId: f.startTaskID})
		return msgID, out, err
	default:
		return 0, nil, assert.AnError
	}
}

type unifiedFakeResolver struct {
	callers map[string]transport.SessionCaller
}

func (r *unifiedFakeResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.callers[agentID]
	return c, ok
}

type unifiedFixture struct {
	db      *gorm.DB
	svcCtx  *svc.ServiceContext
	caller  *unifiedFakeCaller
	audit   *audit.AuditService
	approvl *approvals.MemStore
}

func newUnifiedFixture(t *testing.T) *unifiedFixture {
	t.Helper()
	dsn := "file:unified_invoke_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	store := reg.NewStore()
	resolver := &unifiedFakeResolver{callers: map[string]transport.SessionCaller{}}
	dispatcher := dispatch.NewDispatcher(store)
	dispatcher.SetSessionResolver(resolver)

	policyManager, err := policymgr.NewManager(db, "")
	require.NoError(t, err)

	approvalsStore := approvals.NewMemStore()
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)

	execWriter := executionlog.NewWriter(db, executionlog.Config{
		Enabled:       true,
		FlushInterval: time.Millisecond,
		BatchSize:     1,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go execWriter.Run(ctx)
	t.Cleanup(func() { execWriter.Stop() })

	f := &unifiedFixture{
		db:      db,
		caller:  &unifiedFakeCaller{},
		audit:   auditSvc,
		approvl: approvalsStore,
	}
	f.svcCtx = &svc.ServiceContext{
		DB:                 db,
		AdminModel:         model.NewAdminModel(db),
		RoleModel:          model.NewRoleModel(db),
		FunctionModel:      model.NewFunctionModel(db),
		RegistryStore:      store,
		PolicyManager:      policyManager,
		ApprovalsStore:     approvalsStore,
		AuditService:       auditSvc,
		Dispatcher:         dispatcher,
		ExecutionLogWriter: execWriter,
	}
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo",
		Env:      "prod",
		Addr:     "127.0.0.1:9300",
		ExpireAt: time.Now().Add(5 * time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"demo.echo":   {Enabled: true},
			"danger.fn":   {Enabled: true},
			"disabled.fn": {Enabled: true},
		},
	}))
	f.caller.invokePayload = []byte(`{"echo":true}`)
	resolver.callers["agent-1"] = f.caller

	admin := &model.Admin{Username: "opuser", Nickname: "opuser", Status: 1}
	require.NoError(t, f.svcCtx.AdminModel.Create(context.Background(), admin, "pw"))
	role := &model.Role{Name: "admin", Category: "test"}
	require.NoError(t, f.svcCtx.RoleModel.Create(context.Background(), role))
	require.NoError(t, f.svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID))
	return f
}

func (f *unifiedFixture) ctxFor(username string) context.Context {
	ctx := context.WithValue(context.Background(), "username", username)
	return svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
}

// invokeBoth 以两个入口各触发一次调用，返回 (apiResp, logicResp, err)。
func invokeBoth(t *testing.T, f *unifiedFixture, username, id, payload string) (
	*functionapi.FunctionInvokeResponse, *logicfunction.FunctionInvokeResponse, error) {
	t.Helper()
	apiResp, _ := functionapi.NewService(f.svcCtx).FunctionInvoke(f.ctxFor(username), &functionapi.FunctionInvokeRequest{
		ID: id, Payload: []byte(payload),
	})
	logicLogic := logicfunction.NewFunctionInvokeLogic(f.ctxFor(username), f.svcCtx)
	logicResp, logicErr := logicLogic.FunctionInvoke(&logicfunction.FunctionInvokeRequest{
		ID: id, Payload: []byte(payload), GameID: "demo", Env: "prod",
	})
	return apiResp, logicResp, logicErr
}

// 审批面：高危策略要求审批时，两个入口都返回 pending 审批单而非执行结果，
// 审批单字段（actor/route/scope/payload）一致，且不发往 agent。
func TestUnifiedInvoke_ApprovalFaceEquivalence(t *testing.T) {
	f := newUnifiedFixture(t)
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.fn", &policymgr.Policy{
		FunctionID:       "danger.fn",
		AllowedRoles:     []string{},
		RequireApproval:  true,
		RequireAudit:     true,
		ApprovalWorkflow: "two_person",
		DefaultRiskLevel: "high",
	}))

	apiResp, logicResp, err := invokeBoth(t, f, "opuser", "danger.fn", `{"n":1}`)
	require.NoError(t, err)

	for _, resp := range []*functionapi.FunctionInvokeResponse{apiResp} {
		require.NotNil(t, resp)
		assert.True(t, resp.ApprovalRequired)
		assert.Equal(t, "two_person", resp.ApprovalWorkflow)
		assert.NotEmpty(t, resp.ApprovalID)
		assert.Nil(t, resp.Result)
	}
	require.NotNil(t, logicResp)
	assert.True(t, logicResp.ApprovalRequired)
	assert.Equal(t, "two_person", logicResp.ApprovalWorkflow)
	assert.NotEmpty(t, logicResp.ApprovalID)
	assert.Nil(t, logicResp.Result)
	assert.NotEqual(t, apiResp.ApprovalID, logicResp.ApprovalID)

	for _, approvalID := range []string{apiResp.ApprovalID, logicResp.ApprovalID} {
		stored, err := f.approvl.Get(approvalID)
		require.NoError(t, err)
		assert.Equal(t, "pending", stored.State)
		assert.Equal(t, "danger.fn", stored.FunctionID)
		assert.Equal(t, "demo", stored.GameID)
		assert.Equal(t, "prod", stored.Env)
		assert.Equal(t, "opuser", stored.Actor)
		assert.Equal(t, "lb", stored.Route)
		assert.JSONEq(t, `{"n":1}`, string(stored.Payload))
	}

	// 两个入口都不应触达 agent。
	assert.Empty(t, f.caller.requests)

	// 审批创建均按策略留审计。
	records, total, err := f.audit.List(audit.AuditFilter{}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 2)
	createdByEntry := map[string]bool{}
	for _, rec := range records {
		if rec.EventType == audit.EventApprovalCreated {
			createdByEntry[rec.Resource.ID] = true
		}
	}
	assert.True(t, createdByEntry[apiResp.ApprovalID], "api entry must audit approval creation")
	assert.True(t, createdByEntry[logicResp.ApprovalID], "logic entry must audit approval creation")
}

// 禁用面：禁用函数在两个入口都被 409 function_disabled 拒绝，且不产生执行。
func TestUnifiedInvoke_DisabledFaceEquivalence(t *testing.T) {
	f := newUnifiedFixture(t)
	require.NoError(t, f.svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "disabled.fn",
		GameID:     "demo",
		Status:     model.StatusDisabled,
	}))

	apiResp, logicResp, err := invokeBoth(t, f, "opuser", "disabled.fn", `{}`)
	require.Error(t, err)

	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 409, codeErr.Code)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	assert.Nil(t, apiResp)
	assert.Nil(t, logicResp)
	assert.Empty(t, f.caller.requests)
}

// 留痕面：两个入口的成功调用都写 execution_logs（source=invoke、生效 scope、
// actor 与请求载荷），失败调用写失败状态。
func TestUnifiedInvoke_ExecutionLogFaceEquivalence(t *testing.T) {
	f := newUnifiedFixture(t)

	_, _, err := invokeBoth(t, f, "opuser", "demo.echo", `{"x":1}`)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, f.db.Model(&model.ExecutionLog{}).
			Where("source = ?", executionlog.SourceInvoke).Count(&count).Error)
		return count >= 2
	}, 3*time.Second, 20*time.Millisecond)

	var logs []model.ExecutionLog
	require.NoError(t, f.db.Where("source = ?", executionlog.SourceInvoke).
		Order("id ASC").Find(&logs).Error)
	require.Len(t, logs, 2)
	for _, entry := range logs {
		assert.Equal(t, "demo", entry.GameID)
		assert.Equal(t, "prod", entry.Env)
		assert.Equal(t, "demo.echo", entry.FunctionID)
		assert.Equal(t, "opuser", entry.Actor)
		assert.Equal(t, executionlog.StatusOK, entry.Status)
		assert.JSONEq(t, `{"x":1}`, string(entry.RequestPayload))
	}

	// 失败调用（无可达 agent）→ 两个入口各留一条失败留痕。
	f.caller.err = assert.AnError
	_, _, err = invokeBoth(t, f, "opuser", "demo.echo", `{}`)
	require.Error(t, err)
	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, f.db.Model(&model.ExecutionLog{}).
			Where("source = ? AND status = ?", executionlog.SourceInvoke, executionlog.StatusFail).
			Count(&count).Error)
		return count >= 2
	}, 3*time.Second, 20*time.Millisecond)
}

// 成功路径等价：两个入口返回一致的业务结果与生效 scope 元数据。
func TestUnifiedInvoke_SuccessEquivalence(t *testing.T) {
	f := newUnifiedFixture(t)

	apiResp, logicResp, err := invokeBoth(t, f, "opuser", "demo.echo", `{"x":1}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"echo":true}`, string(apiResp.Result))
	assert.JSONEq(t, `{"echo":true}`, string(logicResp.Result))
	assert.Equal(t, "demo", apiResp.ExecutionMetadata["gameId"])
	assert.Equal(t, "prod", apiResp.ExecutionMetadata["env"])
	assert.Equal(t, "opuser", apiResp.ExecutionMetadata["actor"])

	// 两次调用（两入口）均到达 agent。
	require.Len(t, f.caller.requests, 2)
	assert.Equal(t, "demo.echo", f.caller.requests[0].GetFunctionId())
	assert.Equal(t, "demo.echo", f.caller.requests[1].GetFunctionId())
}
