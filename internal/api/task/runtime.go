package task

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// TaskRuntime is the task execution domain interface. It decouples task
// handlers/services from the full *svc.ServiceContext, exposing only the task
// execution surface (dispatch + function metadata + task store).
//
// This is the pilot ServiceContext split called out in todo P1: task depends
// on TaskRuntime, not *ServiceContext, so it can be tested with a narrow mock
// and the dependency surface is explicit. Auth/scope still flows through
// svc.ServiceContext during the pilot.
type TaskRuntime interface {
	// StartTask dispatches an async task and returns the assigned task ID.
	StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error)
	// CancelTask forwards a cancellation request to the running task's agent.
	CancelTask(ctx context.Context, taskID string) error
	// FindFunctionContract resolves an executable function from the scoped
	// registration source of truth before a task is dispatched.
	FindFunctionContract(ctx context.Context, gameID, env, functionID string) (*model.FunctionContract, error)

	// Task store operations.
	GetRun(ctx context.Context, taskID string) (*model.TaskRun, error)
	ListRuns(ctx context.Context, opts model.ListTasksOptions) ([]model.TaskRun, int64, error)
	ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]model.TaskEvent, error)
	UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error
	UpdateRunIfStatusNotIn(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}) (bool, error)
	AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error
}

// taskRuntime adapts *svc.ServiceContext to TaskRuntime.
type taskRuntime struct {
	dispatcher    *dispatch.Dispatcher
	contractModel *model.FunctionContractModel
	store         *tasks.Store
}

// NewTaskRuntime builds a TaskRuntime from the dispatch/function/store
// components held by *svc.ServiceContext.
func NewTaskRuntime(svcCtx *svc.ServiceContext) TaskRuntime {
	return &taskRuntime{
		dispatcher:    svcCtx.Dispatcher,
		contractModel: model.NewFunctionContractModel(svcCtx.DB),
		store: tasks.NewStore(
			model.NewTaskRunModel(svcCtx.DB),
			model.NewTaskEventModel(svcCtx.DB),
		),
	}
}

func (r *taskRuntime) StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	if r.dispatcher == nil {
		return nil, fmt.Errorf("task dispatcher not configured")
	}
	return r.dispatcher.StartTaskRequest(ctx, req)
}

func (r *taskRuntime) CancelTask(ctx context.Context, taskID string) error {
	if r.dispatcher == nil {
		return nil
	}
	return r.dispatcher.CancelTask(ctx, taskID)
}

func (r *taskRuntime) FindFunctionContract(ctx context.Context, gameID, env, functionID string) (*model.FunctionContract, error) {
	if r.contractModel == nil {
		return nil, fmt.Errorf("function contract model not configured")
	}
	return r.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
}

func (r *taskRuntime) GetRun(ctx context.Context, taskID string) (*model.TaskRun, error) {
	return r.store.GetRun(ctx, taskID)
}

func (r *taskRuntime) ListRuns(ctx context.Context, opts model.ListTasksOptions) ([]model.TaskRun, int64, error) {
	return r.store.ListRuns(ctx, opts)
}

func (r *taskRuntime) ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]model.TaskEvent, error) {
	return r.store.ListEvents(ctx, taskID, afterSeq)
}

func (r *taskRuntime) UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error {
	return r.store.UpdateRun(ctx, taskID, updates)
}

func (r *taskRuntime) UpdateRunIfStatusNotIn(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}) (bool, error) {
	return r.store.UpdateRunIfStatusNotIn(ctx, taskID, blockedStatuses, updates)
}

func (r *taskRuntime) AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error {
	return r.store.AppendEvent(ctx, taskID, eventType, progress, message, payload)
}
