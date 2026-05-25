package task

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func seedTaskRunForTest(t *testing.T, ctx *svc.ServiceContext, run *model.TaskRun) {
	t.Helper()
	if err := model.NewTaskRunModel(ctx.DB).Create(context.Background(), run); err != nil {
		t.Fatalf("create task run: %v", err)
	}
}

func seedTaskEventForTest(t *testing.T, ctx *svc.ServiceContext, event *model.TaskEvent) {
	t.Helper()
	if err := model.NewTaskEventModel(ctx.DB).Append(context.Background(), event); err != nil {
		t.Fatalf("append task event: %v", err)
	}
}

func setupTaskServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return &svc.ServiceContext{
		DB: db,
	}
}

func TestServiceEvents_AfterSeqAndNextSeq(t *testing.T) {
	svcCtx := setupTaskServiceContext(t)
	service := NewService(svcCtx)

	taskID := "task-events-1"
	seedTaskRunForTest(t, svcCtx, &model.TaskRun{
		TaskID:     taskID,
		FunctionID: "examples.echo.invoke",
		Status:     tasks.StatusRunning,
		Progress:   50,
		Message:    "running",
	})
	seedTaskEventForTest(t, svcCtx, &model.TaskEvent{TaskID: taskID, Seq: 1, Type: "queued", Message: "queued", Payload: []byte("null")})
	seedTaskEventForTest(t, svcCtx, &model.TaskEvent{TaskID: taskID, Seq: 2, Type: "started", Message: "started", Payload: []byte("null")})
	seedTaskEventForTest(t, svcCtx, &model.TaskEvent{TaskID: taskID, Seq: 3, Type: "progress", Message: "progress", Progress: 50, Payload: []byte(`{"step":1}`)})

	resp, err := service.Events(context.Background(), &EventsRequest{
		ID:       taskID,
		AfterSeq: 1,
	})
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if resp.Done {
		t.Fatal("Events() done = true, want false")
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Seq != 2 || resp.Items[1].Seq != 3 {
		t.Fatalf("seqs = [%d %d], want [2 3]", resp.Items[0].Seq, resp.Items[1].Seq)
	}
	if resp.NextSeq != 4 {
		t.Fatalf("nextSeq = %d, want 4", resp.NextSeq)
	}
}

func TestServiceEvents_DoneWhenRunTerminalAndNoNewEvents(t *testing.T) {
	svcCtx := setupTaskServiceContext(t)
	service := NewService(svcCtx)

	taskID := "task-events-2"
	seedTaskRunForTest(t, svcCtx, &model.TaskRun{
		TaskID:     taskID,
		FunctionID: "examples.echo.invoke",
		Status:     tasks.StatusSucceeded,
		Progress:   100,
		Message:    "done",
	})
	seedTaskEventForTest(t, svcCtx, &model.TaskEvent{TaskID: taskID, Seq: 1, Type: "completed", Message: "done", Progress: 100, Payload: []byte(`{"ok":true}`)})

	resp, err := service.Events(context.Background(), &EventsRequest{
		ID:       taskID,
		AfterSeq: 1,
	})
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if !resp.Done {
		t.Fatal("Events() done = false, want true")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(resp.Items))
	}
	if resp.NextSeq != 1 {
		t.Fatalf("nextSeq = %d, want 1", resp.NextSeq)
	}
}
