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

func main() {
	fmt.Println("=== Task Result Tracking Demo ===")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}); err != nil {
		log.Fatalf("failed to migrate task tables: %v", err)
	}

	store := tasks.NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db))
	ctx := context.Background()

	fmt.Println("\n1. Simulating task execution...")
	taskRuns := []model.TaskRun{
		{TaskID: "task-001", FunctionID: "demo.longTask", AgentID: "agent-1", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 1})},
		{TaskID: "task-002", FunctionID: "demo.longTask", AgentID: "agent-1", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 2})},
		{TaskID: "task-003", FunctionID: "demo.longTask", AgentID: "agent-2", Status: "queued", InputPayload: tasks.JSONPayload(map[string]any{"n": 3})},
	}

	for i := range taskRuns {
		if err := store.CreateRun(ctx, &taskRuns[i]); err != nil {
			log.Fatalf("failed to create task run: %v", err)
		}
		if err := store.AppendEvent(ctx, taskRuns[i].TaskID, tasks.EventQueued, 0, "task queued", nil); err != nil {
			log.Fatalf("failed to append queued event: %v", err)
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
			log.Fatalf("failed to update task run: %v", err)
		}
		eventType := tasks.EventProgress
		if update.status == "failed" {
			eventType = tasks.EventFailed
		}
		if err := store.AppendEvent(ctx, update.taskID, eventType, update.progress, update.message, tasks.JSONPayload(map[string]any{"status": update.status})); err != nil {
			log.Fatalf("failed to append progress event: %v", err)
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
			log.Fatalf("failed to complete task run: %v", err)
		}
		if err := store.AppendEvent(ctx, completion.taskID, tasks.EventCompleted, 100, "task completed", tasks.JSONPayload(completion.result)); err != nil {
			log.Fatalf("failed to append completion event: %v", err)
		}
		fmt.Printf("   Completed task %s\n", completion.taskID)
	}

	fmt.Println("\n5. Event history for task-001:")
	events, err := store.ListEvents(ctx, "task-001", 0)
	if err != nil {
		log.Fatalf("failed to list task events: %v", err)
	}
	for _, event := range events {
		fmt.Printf("   #%d %s %d%% - %s\n", event.Seq, event.Type, event.Progress, event.Message)
	}

	fmt.Println("\n=== Demo Complete ===")
}
