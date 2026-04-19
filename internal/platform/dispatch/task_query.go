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
			Seq:      evt.Seq,
			Type:     evt.Type,
			Progress: evt.Progress,
			Message:  evt.Message,
			Payload:  evt.Payload,
		}
	}
	return result, nil
}

// GetRun returns the task run for the given task ID.
func (a *TaskEventQueryAdapter) GetRun(ctx context.Context, taskID string) (*sdkv1.TaskRun, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}

	run, err := a.runs.FindByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	return &sdkv1.TaskRun{
		TaskId:            run.TaskID,
		FunctionId:        run.FunctionID,
		AgentId:           run.AgentID,
		ProviderId:        run.ProviderID,
		GameId:            run.GameID,
		Env:               run.Env,
		Status:            run.Status,
		Progress:          run.Progress,
		Message:           run.Message,
		InputPayload:      run.InputPayload,
		ResultPayload:     run.ResultPayload,
		ErrorMessage:      run.ErrorMessage,
		TraceId:           run.TraceID,
		IdempotencyKey:    run.IdempotencyKey,
	}, nil
}
