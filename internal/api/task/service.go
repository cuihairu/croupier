package task

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
)

type Service struct {
	svcCtx  *svc.ServiceContext
	runtime TaskRuntime
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx:  svcCtx,
		runtime: NewTaskRuntime(svcCtx),
	}
}

func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	items, total, err := s.runtime.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: model.NewPagination(req.Page, req.Size),
		FunctionID:        req.FunctionID,
		Status:            req.Status,
		GameID:            svc.ResolveGameID(ctx, req.GameID),
		Env:               svc.ResolveEnv(ctx, req.Env),
	})
	if err != nil {
		return nil, err
	}
	result := make([]Item, 0, len(items))
	for i := range items {
		result = append(result, buildItem(&items[i]))
	}
	return &ListResponse{Items: result, Total: int(total)}, nil
}

func (s *Service) Start(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.FunctionID)
	if err != nil {
		return nil, err
	}
	if _, err := s.runtime.FindFunction(ctx, functionID); err != nil {
		return nil, err
	}

	// Apply the same authorization as the function-invoke path so /tasks
	// cannot be used to bypass function-level RBAC.
	admin, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(ctx, s.svcCtx, roles)
	if err != nil {
		return nil, err
	}

	gameID := svc.ResolveGameID(ctx, req.GameID)
	env := svc.ResolveEnv(ctx, req.Env)
	if err := utils.RequireGameEnvScope(ctx, s.svcCtx, admin.ID, roleNames, gameID, env); err != nil {
		return nil, err
	}
	if err := utils.CheckInvokePermission(ctx, s.svcCtx, roleNames, permIDs, functionID, gameID, env); err != nil {
		return nil, err
	}

	payloadObj := req.Params
	if payloadObj == nil {
		payloadObj = map[string]interface{}{}
	}
	payload, err := json.Marshal(payloadObj)
	if err != nil {
		return nil, err
	}

	metadata := map[string]string{}
	if gameID != "" {
		metadata["game_id"] = gameID
	}
	if env != "" {
		metadata["env"] = env
	}
	// 记录操作者
	metadata["actor"] = admin.Username

	// Dispatch through the same path as mode=task function invocation: the
	// dispatcher generates the task ID, creates the task_runs row, and forwards
	// to the agent. This unifies the async entry points so /tasks no longer
	// leaves rows stranded in "queued".
	resp, err := s.runtime.StartTask(ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
	if err != nil {
		return nil, err
	}
	return &StartResponse{TaskID: resp.GetTaskId(), Status: tasks.StatusDispatching}, nil
}

func (s *Service) Detail(ctx context.Context, req *DetailRequest) (*DetailResponse, error) {
	taskID := strings.TrimSpace(req.ID)
	if taskID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}
	run, err := s.runtime.GetRun(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildDetail(run), nil
}

func (s *Service) Events(ctx context.Context, req *EventsRequest) (*EventsResponse, error) {
	taskID := strings.TrimSpace(req.ID)
	if taskID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}
	run, err := s.runtime.GetRun(ctx, taskID)
	if err != nil {
		return nil, err
	}
	events, err := s.runtime.ListEvents(ctx, taskID, req.AfterSeq)
	if err != nil {
		return nil, err
	}
	items := make([]EventItem, 0, len(events))
	var nextSeq int64 = req.AfterSeq
	for i := range events {
		payload := decodePayload(events[i].Payload)
		items = append(items, EventItem{
			Seq:       events[i].Seq,
			Type:      events[i].Type,
			Progress:  events[i].Progress,
			Message:   events[i].Message,
			Payload:   payload,
			CreatedAt: utils.FormatTimestamp(events[i].CreatedAt),
		})
		if events[i].Seq >= nextSeq {
			nextSeq = events[i].Seq + 1
		}
	}
	done := run.Status == tasks.StatusSucceeded || run.Status == tasks.StatusFailed || run.Status == tasks.StatusCancelled || run.Status == tasks.StatusTimedOut
	return &EventsResponse{Items: items, NextSeq: nextSeq, Done: done}, nil
}

func (s *Service) Cancel(ctx context.Context, req *CancelRequest) error {
	taskID := strings.TrimSpace(req.ID)
	if taskID == "" {
		return errorx.NewBadRequest("任务ID不能为空")
	}
	now := time.Now()
	if err := s.runtime.UpdateRun(ctx, taskID, map[string]interface{}{
		"status":              tasks.StatusCancelRequested,
		"message":             "已请求取消任务",
		"cancel_requested_at": &now,
	}); err != nil {
		return err
	}
	if err := s.runtime.AppendEvent(ctx, taskID, tasks.EventCancelRequested, 0, "已请求取消任务", []byte("null")); err != nil {
		return err
	}

	// Forward the cancellation to the agent so the running task actually
	// stops. Without this, /tasks/cancel only updates the DB row and the
	// agent keeps executing — leaving REST cancellation a no-op against the
	// live task. Best-effort: if the agent is unreachable the row still
	// reflects the requested-cancel state so operators and the SSE stream
	// see the intent.
	_ = s.runtime.CancelTask(ctx, taskID)
	return nil
}

func buildItem(run *model.TaskRun) Item {
	item := Item{
		ID:         run.TaskID,
		FunctionID: run.FunctionID,
		Status:     run.Status,
		Progress:   run.Progress,
		Message:    run.Message,
		GameID:     run.GameID,
		Env:        run.Env,
		AgentID:    run.AgentID,
		Actor:      run.Actor,
		Addr:       run.Addr,
		TraceID:    run.TraceID,
		CreatedAt:  utils.FormatTimestamp(run.CreatedAt),
		Error:      run.ErrorMessage,
	}
	if run.StartedAt != nil {
		item.StartedAt = utils.FormatTimestamp(*run.StartedAt)
	}
	if run.FinishedAt != nil {
		item.FinishedAt = utils.FormatTimestamp(*run.FinishedAt)
	}
	// 计算耗时
	if run.StartedAt != nil && run.FinishedAt != nil {
		item.DurationMs = run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}
	return item
}

func buildDetail(run *model.TaskRun) *DetailResponse {
	resp := &DetailResponse{
		ID:         run.TaskID,
		FunctionID: run.FunctionID,
		Status:     run.Status,
		Progress:   run.Progress,
		Message:    run.Message,
		GameID:     run.GameID,
		Env:        run.Env,
		AgentID:    run.AgentID,
		Actor:      run.Actor,
		Addr:       run.Addr,
		TraceID:    run.TraceID,
		Error:      run.ErrorMessage,
		CreatedAt:  utils.FormatTimestamp(run.CreatedAt),
		UpdatedAt:  utils.FormatTimestamp(run.UpdatedAt),
		Result:     decodePayload(run.ResultPayload),
	}
	if run.StartedAt != nil {
		resp.StartedAt = utils.FormatTimestamp(*run.StartedAt)
	}
	if run.FinishedAt != nil {
		resp.FinishedAt = utils.FormatTimestamp(*run.FinishedAt)
	}
	// 计算耗时
	if run.StartedAt != nil && run.FinishedAt != nil {
		resp.DurationMs = run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}
	return resp
}

func decodePayload(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return string(data)
	}
	return out
}
