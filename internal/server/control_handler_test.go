package server

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	gsqlite "github.com/glebarez/sqlite"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func setupControlServiceTaskDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func TestControlServiceHandleTaskEvent_UpdatesRunAndAppendsEvent(t *testing.T) {
	db := setupControlServiceTaskDB(t)
	runModel := model.NewTaskRunModel(db)
	eventModel := model.NewTaskEventModel(db)
	taskStore := tasks.NewStore(runModel, eventModel)

	svc := NewControlService(reg.NewStore(), nil)
	svc.SetTaskStore(taskStore)

	taskID := "task-123"
	if err := runModel.Create(context.Background(), &model.TaskRun{
		TaskID:      taskID,
		FunctionID:  "examples.echo.invoke",
		Status:      tasks.StatusQueued,
		Progress:    0,
		Message:     "queued",
		ResultPayload: model.EncodeTaskPayload(nil),
	}); err != nil {
		t.Fatalf("create task run: %v", err)
	}

	payload := []byte(`{"ok":true}`)
	data, err := proto.Marshal(&sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     string(tasks.EventCompleted),
		Message:  "done",
		Progress: 100,
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	if _, err := svc.handleTaskEvent(context.Background(), data); err != nil {
		t.Fatalf("handleTaskEvent: %v", err)
	}

	run, err := runModel.FindByTaskID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("find task run: %v", err)
	}
	if run.Status != tasks.StatusSucceeded {
		t.Fatalf("status=%q want %q", run.Status, tasks.StatusSucceeded)
	}
	if run.Progress != 100 {
		t.Fatalf("progress=%d want 100", run.Progress)
	}
	if run.Message != "done" {
		t.Fatalf("message=%q want done", run.Message)
	}
	if run.FinishedAt == nil {
		t.Fatal("finished_at should be set")
	}
	if string(run.ResultPayload) != string(payload) {
		t.Fatalf("result payload=%s want %s", string(run.ResultPayload), string(payload))
	}

	events, err := eventModel.ListByTaskID(context.Background(), taskID, 0)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events=%d want 1", len(events))
	}
	if events[0].Type != string(tasks.EventCompleted) {
		t.Fatalf("event type=%q want %q", events[0].Type, tasks.EventCompleted)
	}
	if events[0].Seq != 1 {
		t.Fatalf("event seq=%d want 1", events[0].Seq)
	}
}

func TestControlServiceHandleTaskEvent_FailedSetsErrorMessage(t *testing.T) {
	db := setupControlServiceTaskDB(t)
	runModel := model.NewTaskRunModel(db)
	eventModel := model.NewTaskEventModel(db)
	taskStore := tasks.NewStore(runModel, eventModel)

	svc := NewControlService(reg.NewStore(), nil)
	svc.SetTaskStore(taskStore)

	taskID := "task-fail"
	if err := runModel.Create(context.Background(), &model.TaskRun{
		TaskID:     taskID,
		FunctionID: "examples.echo.invoke",
		Status:     tasks.StatusRunning,
	}); err != nil {
		t.Fatalf("create task run: %v", err)
	}

	data, err := proto.Marshal(&sdkv1.TaskEvent{
		TaskId:   taskID,
		Type:     string(tasks.EventFailed),
		Message:  "boom",
		Progress: 42,
		Payload:  []byte("null"),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	if _, err := svc.handleTaskEvent(context.Background(), data); err != nil {
		t.Fatalf("handleTaskEvent: %v", err)
	}

	run, err := runModel.FindByTaskID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("find task run: %v", err)
	}
	if run.Status != tasks.StatusFailed {
		t.Fatalf("status=%q want %q", run.Status, tasks.StatusFailed)
	}
	if run.ErrorMessage != "boom" {
		t.Fatalf("error_message=%q want boom", run.ErrorMessage)
	}
	if run.FinishedAt == nil {
		t.Fatal("finished_at should be set")
	}
}
