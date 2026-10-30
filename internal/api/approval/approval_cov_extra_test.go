// 覆盖目标：continueApprovedFunction 的成功续跑分支（sync 结果 / async 任务）、
// rejectSelfApproval 的 nil 防御、logSelfApprovalAudit 落审计、Reject 的 store 错误、
// filterApprovalSummaries / filterApprovalSummariesByActor 的 actor/functionID 过滤。
package approval

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var covSharedDBSeq atomic.Int64

// openCovSharedDB 打开共享缓存 sqlite 内存库：多 goroutine/多连接共享同一
// 个 :memory: 库（默认 :memory: 每连接一个，跨连接看不到建表结果）。
func openCovSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("approval_cov_shared_%d", covSharedDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// ---- fake agent 会话（镜像 function 包 invoke_coverage_test 的实现） ----

type covSessionCaller struct {
	invokePayload  []byte
	startTaskID    string
	invokeMetadata map[string]string
	invokeCount    int
}

func (c *covSessionCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	switch msgID {
	case protocol.MsgInvokeRequest:
		c.invokeCount++
		var req sdkv1.InvokeRequest
		if err := proto.Unmarshal(body, &req); err == nil {
			c.invokeMetadata = req.GetMetadata()
		}
		resp := &sdkv1.InvokeResponse{Payload: c.invokePayload}
		out, err := proto.Marshal(resp)
		return msgID, out, err
	case protocol.MsgStartTaskRequest:
		resp := &sdkv1.StartTaskResponse{TaskId: c.startTaskID}
		out, err := proto.Marshal(resp)
		return msgID, out, err
	default:
		return 0, nil, errors.New("unexpected message")
	}
}

type covResolver struct {
	callers map[string]transport.SessionCaller
}

func (r *covResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	c, ok := r.callers[agentID]
	return c, ok
}

// covContinuationEnv 构造可成功派发的审批续跑环境：
// 管理员 + 角色、注册函数的 agent 会话、内存审批库、假会话通道。
func covContinuationEnv(t *testing.T, mode string) (*Service, context.Context) {
	s, ctx, _ := covContinuationEnvSQL(t, mode, false)
	return s, ctx
}

// covContinuationEnvSQL 与 covContinuationEnv 同构，但审批库可切换为
// SQL 存储（文件库，支持并发与跨重开），返回 caller 供断言调用次数。
func covContinuationEnvSQL(t *testing.T, mode string, sqlStore bool) (*Service, context.Context, *covSessionCaller) {
	t.Helper()
	db := openCovSharedDB(t)
	err := model.AutoMigrate(db)
	require.NoError(t, err)

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	approver := &model.Admin{Username: "approver", Nickname: "Approver", Status: 1}
	err = adminModel.Create(context.Background(), approver, "pw")
	require.NoError(t, err)
	role := &model.Role{Name: "operator"}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), approver.ID, role.ID)
	require.NoError(t, err)

	store := reg.NewStore()
	err = store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-cov", GameID: "covgame", Env: "prod",
		Addr: "127.0.0.1:9000", Functions: map[string]reg.FunctionMeta{
			"cov.echo": {Enabled: true, Version: "1.0.0"},
		},
		ExpireAt: time.Now().Add(5 * time.Minute), LastSeen: time.Now(),
	})
	require.NoError(t, err)
	resolver := &covResolver{callers: map[string]transport.SessionCaller{}}
	dispatcher := dispatch.NewDispatcher(store)
	dispatcher.SetSessionResolver(resolver)

	var approvalsStore approvals.Store
	if sqlStore {
		approvalDB := openCovSharedDB(t)
		sqlS, err := approvals.NewSQLStore(approvalDB)
		require.NoError(t, err)
		approvalsStore = sqlS
	} else {
		approvalsStore = approvals.NewMemStore()
	}

	svcCtx := &svc.ServiceContext{
		DB:             db,
		AdminModel:     adminModel,
		RoleModel:      roleModel,
		RegistryStore:  store,
		Dispatcher:     dispatcher,
		ApprovalsStore: approvalsStore,
	}
	caller := &covSessionCaller{invokePayload: []byte(`{"echo":true}`), startTaskID: "task-cov-9"}
	resolver.callers["agent-cov"] = caller

	_, err = svcCtx.ApprovalsStore.Create(&approvals.Approval{
		ID: "ap-cov", State: "pending", FunctionID: "cov.echo",
		GameID: "covgame", Env: "prod", Actor: "tester",
		Payload: []byte(`{"x":1}`), Mode: mode,
	})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "approver")
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "covgame", Env: "prod"})
	return NewService(svcCtx), ctx, caller
}

// Approve 成功续跑（同步）：ResultKind=sync 且携带结果。
func TestApproveCov_ContinuationSyncSuccess(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")

	resp, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "approved", resp.State)
	assert.True(t, resp.Continuation)
	assert.Equal(t, "sync", resp.ResultKind)
	assert.Empty(t, resp.TaskID)
	assert.JSONEq(t, `{"echo":true}`, string(resp.Result))
}

// ARCH-002 D7: the durable claim must survive a service restart and travel to
// the game side so it can dedupe/fence on (approvalId, approvalClaimToken).
func TestApproveCov_ContinuationCarriesDispatchClaim(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")

	// Approve the pending record first, then simulate a restart by reading the
	// record back through the store and claim it explicitly.
	stored, err := s.svcCtx.ApprovalsStore.Get("ap-cov")
	require.NoError(t, err)
	require.Equal(t, "pending", stored.State)

	_, err = s.svcCtx.ApprovalsStore.Approve(stored.ID, "approver")
	require.NoError(t, err)

	token, err := approvals.NewClaimToken()
	require.NoError(t, err)
	digest := approvals.PayloadDigest(stored)
	claimed, err := s.svcCtx.ApprovalsStore.ClaimForDispatch(stored.ID, token, digest)
	require.NoError(t, err)
	require.Equal(t, token, claimed.ClaimToken)

	_, err = s.continueApprovedFunction(ctx, claimed)
	require.NoError(t, err)

	caller := s.svcCtx.Dispatcher.SessionResolver().(*covResolver).callers["agent-cov"].(*covSessionCaller)
	require.Equal(t, token, caller.invokeMetadata["approvalClaimToken"])
	require.Equal(t, digest, caller.invokeMetadata["approvalPayloadDigest"])
	require.Equal(t, stored.ID, caller.invokeMetadata["approvalId"])

	_, err = s.svcCtx.ApprovalsStore.ClaimForDispatch(stored.ID, "different-token", digest)
	require.ErrorIs(t, err, approvals.ErrAlreadyReviewed)
}

// Approve 成功续跑（异步任务）：ResultKind=task 且携带任务 ID。
func TestApproveCov_ContinuationTaskSuccess(t *testing.T) {
	s, ctx := covContinuationEnv(t, "async")

	resp, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "approved", resp.State)
	assert.True(t, resp.Continuation)
	assert.Equal(t, "task", resp.ResultKind)
	assert.Equal(t, "task-cov-9", resp.TaskID)
}

// ARCH-002 D7：并发 N 次批准同一 pending 审批，仅一个请求赢得领取并恰好
// 执行一次续跑（invoke 计数 == 1），其余全部 409 且不产生任何续跑副作用。
func TestApproveCov_ConcurrentApprovalRunsExactlyOnce(t *testing.T) {
	s, ctx, caller := covContinuationEnvSQL(t, "", true)

	const n = 6
	results := make(chan error, n)
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start.Wait()
			_, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
			results <- err
		}()
	}
	start.Done()
	wg.Wait()

	successes, conflicts := 0, 0
	for i := 0; i < n; i++ {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		var ce *errorx.CodeError
		require.True(t, errors.As(err, &ce), "非冲突错误: %v", err)
		assert.Equal(t, 409, ce.Code)
		conflicts++
	}
	assert.Equal(t, 1, successes, "exactly one approve must win")
	assert.Equal(t, n-1, conflicts, "all losers must 409")
	assert.Equal(t, 1, caller.invokeCount, "continuation must invoke the game exactly once")
}

// ---- rejectSelfApproval / logSelfApprovalAudit ----

func TestRejectSelfApprovalCov_NilGuards(t *testing.T) {
	var nilService *Service
	assert.NoError(t, nilService.rejectSelfApproval(context.Background(), "op", &approvals.Approval{ID: "x"}, "approve"))

	s := NewService(&svc.ServiceContext{})
	assert.NoError(t, s.rejectSelfApproval(context.Background(), "op", nil, "reject"))
}

func TestLogSelfApprovalAuditCov_WritesAudit(t *testing.T) {
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	s := NewService(&svc.ServiceContext{AuditService: auditSvc})

	s.logSelfApprovalAudit(context.Background(), "alice", &approvals.Approval{
		ID: "ap-audit", FunctionID: "cov.fn", Actor: "alice", GameID: "covgame", Env: "prod",
	}, "approve", "blocked")

	items, total, err := auditSvc.List(audit.AuditFilter{}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	assert.NotEmpty(t, items)
}

// ---- Reject 的 store.Reject 错误 ----

func TestRejectCov_StoreRejectError(t *testing.T) {
	record := &approvals.Approval{ID: "ap-reject", State: "pending", Actor: "tester"}
	s := NewService(&svc.ServiceContext{ApprovalsStore: &stubStore{record: record}})

	// 带登录态且非本人 → 越过自审拦截，命中 stubStore 的 Reject 错误。
	resp, err := s.Reject(context.WithValue(context.Background(), "username", "admin"),
		&ApprovalRejectRequest{ID: "ap-reject", Reason: "no"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// ---- filterApprovalSummaries / ByActor ----

func TestFilterApprovalSummariesCov(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", Actor: "alice", FunctionID: "fn.a", State: "pending"},
		{ID: "2", Actor: "Bob", FunctionID: "fn.b", State: "approved"},
		{ID: "3", Actor: "alice", FunctionID: "fn.b", State: "PENDING"},
	}

	// actor/functionID/state 组合过滤：大小写不敏感、actor 前后空白容忍。
	got := filterApprovalSummaries(items, " alice ", "FN.B", "pending")
	require.Len(t, got, 1)
	assert.Equal(t, "3", got[0].ID)

	// 无过滤条件 → 原样返回。
	got = filterApprovalSummaries(items, "", "", "")
	assert.Len(t, got, 3)

	// actor 不匹配 → 空集。
	got = filterApprovalSummaries(items, "nobody", "", "")
	assert.Empty(t, got)

	// functionID 不匹配 → 空集。
	got = filterApprovalSummaries(items, "", "ghost.fn", "")
	assert.Empty(t, got)

	// state 不匹配 → 空集。
	got = filterApprovalSummaries(items, "", "", "missing")
	assert.Empty(t, got)
}
