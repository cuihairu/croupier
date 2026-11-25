// Coverage targets: the successful continuation branches of
// continueApprovedFunction (sync result / async task), the nil guards of
// rejectSelfApproval, logSelfApprovalAudit writing to the audit log, Reject's
// store errors, and the actor/functionID filtering of filterApprovalSummaries
// / filterApprovalSummariesByActor.
package approval

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// openCovSharedDB opens a shared-cache sqlite in-memory DB: multiple
// goroutines/connections share one :memory: database (by default :memory: is
// one database per connection, so table-creation results are invisible
// across connections).
func openCovSharedDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("approval_cov_shared_%d", covSharedDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// ---- Fake agent sessions (mirrors the function package's invoke_coverage_test) ----

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

// covContinuationEnv builds an approval continuation environment where
// dispatch succeeds: admin + role, an agent session with a registered
// function, an in-memory approval store, and fake session channels.
func covContinuationEnv(t *testing.T, mode string) (*Service, context.Context) {
	s, ctx, _ := covContinuationEnvSQL(t, mode, false)
	return s, ctx
}

// covContinuationEnvSQL is isomorphic to covContinuationEnv, but the
// approval store can be switched to SQL storage (file-backed DB, supporting
// concurrency and reopening across restarts); it returns the caller for
// call-count assertions.
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
	// ARCH-024 (R30-01): Govern now runs CheckInvokePermission, so the
	// approver needs function:invoke (operator has no admin bypass). The
	// approval continuation re-authorizes as the approver, and a missing
	// permission causes a fail-closed refusal -- granting it preserves the
	// original test intent of "operator can approve and continue".
	grantApprovalPermission(t, db, role.ID, "function:invoke")

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
		// ARCH-024 (R30-01): Govern now runs CheckInvokePermission, and a
		// nil FunctionModel causes a fail-closed refusal (403). Mirror the
		// task-service test by injecting a FunctionModel so the approval
		// continuation can reach the dispatch path.
		FunctionModel: model.NewFunctionModel(db),
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

// grantApprovalPermission binds a permission ID to a role (create the
// Permission row first, then write role_permissions) so Govern's default
// CheckInvokePermission fallback (function:invoke) matches. Mirrors the
// console test's grantConsolePermission.
func grantApprovalPermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
	t.Helper()
	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return
	}
	parts := strings.SplitN(permissionID, ":", 2)
	action := "*"
	if len(parts) == 2 {
		action = parts[1]
	}
	permission := model.Permission{
		ID:       permissionID,
		Name:     permissionID,
		Resource: parts[0],
		Action:   action,
		Category: "dashboard",
	}
	require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error)
}

// Approve successful continuation (sync): ResultKind=sync with a payload.
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
	// record back through the store and claiming it explicitly.
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

// Approve successful continuation (async task): ResultKind=task with a task
// ID.
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

// ARCH-002 D7: N concurrent approvals of the same pending approval; only one
// request wins the claim and runs the continuation exactly once (invoke
// count == 1), all the rest are 409 and produce no continuation side effects.
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

// ---- Reject's store.Reject error ----

func TestRejectCov_StoreRejectError(t *testing.T) {
	record := &approvals.Approval{ID: "ap-reject", State: "pending", Actor: "tester"}
	s := NewService(&svc.ServiceContext{ApprovalsStore: &stubStore{record: record}})

	// Logged in and not the same actor -> past the self-review block and
	// hits the stubStore's Reject error.
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

	// actor/functionID/state combined filtering: case-insensitive and
	// tolerant of actor surrounding whitespace.
	got := filterApprovalSummaries(items, " alice ", "FN.B", "pending")
	require.Len(t, got, 1)
	assert.Equal(t, "3", got[0].ID)

	// No filter -> returned as-is.
	got = filterApprovalSummaries(items, "", "", "")
	assert.Len(t, got, 3)

	// actor does not match -> empty set.
	got = filterApprovalSummaries(items, "nobody", "", "")
	assert.Empty(t, got)

	// functionID does not match -> empty set.
	got = filterApprovalSummaries(items, "", "ghost.fn", "")
	assert.Empty(t, got)

	// state does not match -> empty set.
	got = filterApprovalSummaries(items, "", "", "missing")
	assert.Empty(t, got)
}
