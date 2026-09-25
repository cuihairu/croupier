package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// demoDSN 是演示使用的 SQLite DSN；抽成包级变量仅为让 main 的错误分支
// 可在测试子进程中被触发，正常运行取值与原实现一致（:memory:）。
var demoDSN = ":memory:"

func main() {
	if err := run(demoDSN); err != nil {
		log.Fatal(err)
	}
}

// taskStore 是演示所需的存储能力子集；抽出接口仅为让演示主体可注入故障，
// 运行路径仍由 tasks.NewStore 提供真实实现。
type taskStore interface {
	CreateRun(ctx context.Context, run *model.TaskRun) error
	GetRun(ctx context.Context, taskID string) (*model.TaskRun, error)
	UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error
	AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error
	ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]model.TaskEvent, error)
}

func run(dsn string) error {
	fmt.Println("=== Task Result Tracking Demo ===")

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}); err != nil {
		return fmt.Errorf("failed to migrate task tables: %v", err)
	}

	return demonstrate(tasks.NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db)))
}

func demonstrate(store taskStore) error {
	ctx := context.Background()

	fmt.Println("\n1. Simulating task execution...")
	taskRuns := []model.TaskRun{
		{TaskID: "task-001", FunctionID: "demo.longTask", AgentID: "agent-1", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 1})},
		{TaskID: "task-002", FunctionID: "demo.longTask", AgentID: "agent-1", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 2})},
		{TaskID: "task-003", FunctionID: "demo.longTask", AgentID: "agent-2", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 3})},
	}

	for i := range taskRuns {
		if err := store.CreateRun(ctx, &taskRuns[i]); err != nil {
			return fmt.Errorf("failed to create task run: %v", err)
		}
		if err := store.AppendEvent(ctx, taskRuns[i].TaskID, tasks.EventQueued, 0, "task queued", nil); err != nil {
			return fmt.Errorf("failed to append queued event: %v", err)
		}
		fmt.Printf("   Started task %s (status: %s)\n", taskRuns[i].TaskID, taskRuns[i].Status)
	}

	fmt.Println("\n2. Simulating task progress...")
	time.Sleep(100 * time.Millisecond)
	progressUpdates := []struct {
		taskID   string
		status   string
		progress int32
		message  string
		errMsg   string
	}{
		{"task-001", "running", 25, "loading data", ""},
		{"task-002", "running", 50, "processing", ""},
		{"task-003", "failed", 0, "connection timeout", "connection timeout"},
	}

	for _, update := range progressUpdates {
		if err := store.UpdateRun(ctx, update.taskID, map[string]interface{}{
			"status":        update.status,
			"progress":      update.progress,
			"message":       update.message,
			"error_message": update.errMsg,
		}); err != nil {
			return fmt.Errorf("failed to update task run: %v", err)
		}
		eventType := tasks.EventProgress
		if update.status == "failed" {
			eventType = tasks.EventFailed
		}
		if err := store.AppendEvent(ctx, update.taskID, eventType, update.progress, update.message, tasks.JSONPayload(map[string]any{"status": update.status})); err != nil {
			return fmt.Errorf("failed to append progress event: %v", err)
		}
		fmt.Printf("   Updated task %s (status: %s)\n", update.taskID, update.status)
	}

	fmt.Println("\n3. Querying task runs...")
	for _, task := range taskRuns {
		run, err := store.GetRun(ctx, task.TaskID)
		if err != nil {
			fmt.Printf("   Task %s: not found\n", task.TaskID)
			continue
		}
		fmt.Printf("   Task %s: %s (%d%%)\n", run.TaskID, run.Status, run.Progress)
		if run.ErrorMessage != "" {
			fmt.Printf("     Error: %s\n", run.ErrorMessage)
		}
	}

	fmt.Println("\n4. Completing remaining tasks...")
	completions := []struct {
		taskID string
		result map[string]any
	}{
		{"task-001", map[string]any{"result": "success", "value": 42}},
		{"task-002", map[string]any{"result": "success", "value": 100}},
	}
	for _, completion := range completions {
		if err := store.UpdateRun(ctx, completion.taskID, map[string]interface{}{
			"status":         "completed",
			"progress":       int32(100),
			"message":        "task completed",
			"result_payload": tasks.JSONPayload(completion.result),
		}); err != nil {
			return fmt.Errorf("failed to complete task run: %v", err)
		}
		if err := store.AppendEvent(ctx, completion.taskID, tasks.EventCompleted, 100, "task completed", tasks.JSONPayload(completion.result)); err != nil {
			return fmt.Errorf("failed to append completion event: %v", err)
		}
		fmt.Printf("   Completed task %s\n", completion.taskID)
	}

	fmt.Println("\n5. Event history for task-001:")
	events, err := store.ListEvents(ctx, "task-001", 0)
	if err != nil {
		return fmt.Errorf("failed to list task events: %v", err)
	}
	for _, event := range events {
		fmt.Printf("   #%d %s %d%% - %s\n", event.Seq, event.Type, event.Progress, event.Message)
	}

	fmt.Println("\n=== Demo Complete ===")
	return nil
}
