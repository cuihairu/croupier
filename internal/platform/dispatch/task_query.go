package dispatch

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
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
func (a *TaskEventQueryAdapter) ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]*sdkv1.TaskEvent, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return []*sdkv1.TaskEvent{}, nil
	}

	events, err := a.events.ListByTaskID(ctx, taskID, afterSeq)
	if err != nil {
		return nil, err
	}

	result := make([]*sdkv1.TaskEvent, len(events))
	for i, evt := range events {
		result[i] = &sdkv1.TaskEvent{
			TaskId:   evt.TaskID,
			Type:     evt.Type,
			Progress: evt.Progress,
			Message:  evt.Message,
			Payload:  evt.Payload,
		}
	}
	return result, nil
}

// GetRun returns the task run for the given task ID.
// TODO: TaskRun is not yet defined in SDK, returns nil for now.
func (a *TaskEventQueryAdapter) GetRun(ctx context.Context, taskID string) (*sdkv1.TaskEvent, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}

	_, err := a.runs.FindByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Return a placeholder TaskEvent since TaskRun doesn't exist in SDK yet
	return &sdkv1.TaskEvent{
		TaskId: taskID,
		Type:   "completed",
	}, nil
}
