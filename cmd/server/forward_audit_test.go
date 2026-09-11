package main

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
)

// ---- callerFromContext（CallerContext 完整填充）----

// HTTP 链路 ctx（svc.AuthMiddleware 注入的字符串键）身份全量进
// CallerContext：username/roles/adminID + scope。
func TestCallerFromContext_HttpIdentityCarried(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "username", "alice")
	ctx = context.WithValue(ctx, "roles", []string{"ops", "admin"})
	ctx = context.WithValue(ctx, "adminID", uint(42))

	caller := callerFromContext(ctx, map[string]string{"gameId": "demo", "env": "prod"})
	if caller.Username != "alice" || caller.GameID != "demo" || caller.Env != "prod" {
		t.Fatalf("caller = %+v", caller)
	}
	if caller.AdminID != 42 {
		t.Fatalf("admin id = %d, want 42", caller.AdminID)
	}
	if len(caller.Roles) != 2 || caller.Roles[0] != "ops" || caller.Roles[1] != "admin" {
		t.Fatalf("roles = %v", caller.Roles)
	}
}

// scheduler 等后台派发的 ctx 无身份 → 字段留空，仅 scope 从 metadata 进。
func TestCallerFromContext_BackgroundCtxNoIdentity(t *testing.T) {
	caller := callerFromContext(context.Background(), map[string]string{"gameId": "demo", "env": "prod"})
	if caller.Username != "" || caller.AdminID != 0 || len(caller.Roles) != 0 || caller.TraceID != "" {
		t.Fatalf("background ctx should carry no identity: %+v", caller)
	}
	if caller.GameID != "demo" || caller.Env != "prod" {
		t.Fatalf("scope must survive: %+v", caller)
	}
}

// ---- owner 侧转发审计 ----

func newAuditInvoker(fake *fakeLocalDispatcher) (localInvoker, *audit.InMemoryAuditStore) {
	store := audit.NewInMemoryAuditStore()
	return localInvoker{dispatcher: fake, audit: audit.NewAuditService(store, nil)}, store
}

func listAudits(t *testing.T, store *audit.InMemoryAuditStore) []*audit.AuditRecord {
	t.Helper()
	recs, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventFunctionInvoke}}, audit.AuditPage{})
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	return recs
}

// 成功投递落一条 success：actor/资源/details（kind/agent_id/forwarded/
// admin_id/task_id）齐全，与 caller 侧审计同 actor 键。
func TestLocalInvoker_AuditSuccess(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li, store := newAuditInvoker(fake)

	_, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID:    "agent-1",
		FunctionID: "demo.fn",
		Caller:     cluster.CallerContext{Username: "alice", AdminID: 42, GameID: "demo", Env: "prod"},
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	recs := listAudits(t, store)
	if len(recs) != 1 {
		t.Fatalf("audit records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if rec.Outcome != "success" {
		t.Fatalf("outcome = %q", rec.Outcome)
	}
	if rec.Actor.ID != "alice" || rec.Actor.Type != "user" {
		t.Fatalf("actor = %+v, want alice/user (与 caller 侧审计同键)", rec.Actor)
	}
	if rec.Resource.ID != "demo.fn" || rec.Resource.Type != "function" {
		t.Fatalf("resource = %+v", rec.Resource)
	}
	if rec.Details["kind"] != "invoke" || rec.Details["forwarded"] != true {
		t.Fatalf("details = %+v", rec.Details)
	}
	if rec.Details["agent_id"] != "agent-1" || rec.Details["gameId"] != "demo" {
		t.Fatalf("details = %+v", rec.Details)
	}
	if rec.Details["admin_id"] != uint(42) {
		t.Fatalf("admin_id = %v, want 42", rec.Details["admin_id"])
	}
}

// 失败投递（agent 连接断开 → OK=false）落 failure 记录，错误文本进
// outcome 附带信息。
func TestLocalInvoker_AuditFailure(t *testing.T) {
	fake := &fakeLocalDispatcher{startErr: context.DeadlineExceeded}
	li, store := newAuditInvoker(fake)

	_, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		Kind:       cluster.ForwardKindStartTask,
		AgentID:    "agent-1",
		FunctionID: "demo.fn",
		Metadata:   map[string]string{"taskId": "srv-task-1"},
		Caller:     cluster.CallerContext{Username: "bob"},
	})
	if err != nil {
		t.Fatalf("InvokeLocal must return result, not error: %v", err)
	}
	recs := listAudits(t, store)
	if len(recs) != 1 {
		t.Fatalf("audit records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if rec.Outcome != "failure" {
		t.Fatalf("outcome = %q, want failure", rec.Outcome)
	}
	if rec.Details["kind"] != "start_task" {
		t.Fatalf("details kind = %v", rec.Details["kind"])
	}
}

// audit 为 nil（AuditService 未初始化）→ 跳过审计，投递不受影响。
func TestLocalInvoker_AuditNilSkipped(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li := localInvoker{dispatcher: fake}

	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID: "agent-1", FunctionID: "demo.fn",
	})
	if err != nil || !res.OK {
		t.Fatalf("InvokeLocal = %+v, %v; want ok", res, err)
	}
}

// 无身份 caller（背景派发）→ actor 回落 system，不因空 username 丢审计。
func TestLocalInvoker_AuditBackgroundCallerFallsBackToSystem(t *testing.T) {
	fake := &fakeLocalDispatcher{}
	li, store := newAuditInvoker(fake)

	_, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID: "agent-1", FunctionID: "demo.fn",
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	recs := listAudits(t, store)
	if len(recs) != 1 || recs[0].Actor.ID != "system" {
		t.Fatalf("records = %d, actor = %+v; want system", len(recs), recs[0].Actor)
	}
}
