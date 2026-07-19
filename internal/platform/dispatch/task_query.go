package dispatch

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"gorm.io/gorm"
)

// TaskEventQueryAdapter implements TaskEventQuery using model.TaskEventModel and model.TaskRunModel.
type TaskEventQueryAdapter struct {
	events *model.TaskEventModel
	runs   *model.TaskRunModel
}

// NewTaskEventQueryAdapter creates a new TaskEventQuery from database models.
func NewTaskEventQueryAdapter(events *model.TaskEventModel, runs *model.TaskRunModel) *TaskEventQueryAdapter {
	return &TaskEventQueryAdapter{
		events: events,
		runs:   runs,
	}
}

// ListEvents returns task events after the given sequence number.
func (a *TaskEventQueryAdapter) ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]TaskEventRecord, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return []TaskEventRecord{}, nil
	}

	events, err := a.events.ListByTaskID(ctx, taskID, afterSeq)
	if err != nil {
		return nil, err
	}

	result := make([]TaskEventRecord, len(events))
	for i, evt := range events {
		result[i] = TaskEventRecord{
			Seq: evt.Seq,
			Event: &sdkv1.TaskEvent{
				TaskId:   evt.TaskID,
				Type:     evt.Type,
				Progress: evt.Progress,
				Message:  evt.Message,
				Payload:  evt.Payload,
			},
		}
	}
	return result, nil
}

// GetRun returns the task run for the given task ID.
func (a *TaskEventQueryAdapter) GetRun(ctx context.Context, taskID string) (*TaskRunState, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrTaskRunNotFound
	}

	run, err := a.runs.FindByTaskID(ctx, taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskRunNotFound
		}
		return nil, err
	}

	return &TaskRunState{
		TaskID:        run.TaskID,
		Status:        run.Status,
		Progress:      run.Progress,
		Message:       taskRunMessage(run),
		ResultPayload: []byte(run.ResultPayload),
		ErrorMessage:  run.ErrorMessage,
	}, nil
}

func taskRunMessage(run *model.TaskRun) string {
	if run == nil {
		return ""
	}
	if strings.TrimSpace(run.ErrorMessage) != "" {
		return run.ErrorMessage
	}
	return run.Message
}

// TaskRunWriterAdapter implements TaskRunWriter using model.TaskRunModel.
type TaskRunWriterAdapter struct {
	runs *model.TaskRunModel
}

// NewTaskRunWriterAdapter creates a TaskRunWriter backed by the given model.
func NewTaskRunWriterAdapter(runs *model.TaskRunModel) *TaskRunWriterAdapter {
	return &TaskRunWriterAdapter{runs: runs}
}

// CreateRun persists a task_runs row for a newly dispatched task.
func (w *TaskRunWriterAdapter) CreateRun(ctx context.Context, taskID, functionID, agentID, gameID, env, status string, inputPayload []byte) error {
	if w.runs == nil {
		return nil
	}
	run := &model.TaskRun{
		TaskID:       taskID,
		FunctionID:   functionID,
		AgentID:      agentID,
		GameID:       gameID,
		Env:          env,
		Status:       status,
		InputPayload: model.MustJSON(string(inputPayload)),
	}
	return w.runs.Create(ctx, run)
}

// CreateRunWithMeta persists a task_runs row with actor and addr metadata.
func (w *TaskRunWriterAdapter) CreateRunWithMeta(ctx context.Context, taskID, functionID, agentID, gameID, env, status, actor, addr string, inputPayload []byte) error {
	if w.runs == nil {
		return nil
	}
	run := &model.TaskRun{
		TaskID:       taskID,
		FunctionID:   functionID,
		AgentID:      agentID,
		GameID:       gameID,
		Env:          env,
		Status:       status,
		Actor:        actor,
		Addr:         addr,
		InputPayload: model.MustJSON(string(inputPayload)),
	}
	return w.runs.Create(ctx, run)
}
