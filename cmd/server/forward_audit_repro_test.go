package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cluster"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 线上复现（2026-09-11）：双实例转发的 owner 侧审计在 cmd/server 单测
// （InMemoryAuditStore）通过、线上 SQLAuditStore 路径未见落库——本测试用
// 真实 SQL store 走完整 auditForward 调用序列，验证 SQL 路径等价。
func TestLocalInvoker_AuditForwardSQLStorePath(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&audit.AuditModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store, err := audit.NewSQLAuditStore(db)
	if err != nil {
		t.Fatalf("new sql store: %v", err)
	}
	svc := audit.NewAuditService(store, nil)

	li := localInvoker{dispatcher: &fakeLocalDispatcher{}, audit: svc}
	res, err := li.InvokeLocal(context.Background(), &cluster.ForwardedInvoke{
		AgentID:    "agent-2",
		FunctionID: "mail.send",
		Kind:       cluster.ForwardKindStartTask,
		Caller: cluster.CallerContext{
			Username: "admin", AdminID: 1,
			GameID: "demo_game", Env: "dev", TraceID: "trace-1",
		},
	})
	if err != nil {
		t.Fatalf("InvokeLocal: %v", err)
	}
	if !res.OK {
		t.Fatalf("res = %+v, want OK", res)
	}

	var count int64
	db.Model(&audit.AuditModel{}).Where("event_type = ?", string(audit.EventFunctionInvoke)).Count(&count)
	if count != 1 {
		rows := []audit.AuditModel{}
		db.Find(&rows)
		for _, r := range rows {
			fmt.Printf("row: %+v\n", r)
		}
		t.Fatalf("function.invoke rows = %d, want 1", count)
	}
	var model audit.AuditModel
	db.Where("event_type = ?", string(audit.EventFunctionInvoke)).First(&model)
	if model.ActorID != "admin" {
		t.Fatalf("actor = %q, want admin", model.ActorID)
	}
}
