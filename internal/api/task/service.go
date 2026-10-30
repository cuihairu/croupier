package task

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/functioninvoke"
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
	pagination := model.NewPagination(req.Page, req.Size)
	items, total, err := s.runtime.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: pagination,
		FunctionID:        req.FunctionID,
		Status:            req.Status,
		GameID:            svc.ResolveGameID(ctx, req.GameID),
		Env:               svc.ResolveEnv(ctx, req.Env),
		Actor:             req.Actor,
	})
	if err != nil {
		return nil, err
	}
	result := make([]Item, 0, len(items))
	for i := range items {
		result = append(result, buildItem(&items[i]))
	}
	return &ListResponse{Items: result, Total: int(total), Page: pagination.Page, Size: pagination.PageSize}, nil
}

// OperatorOptions 操作者聚合选项（#23）：当前 scope（含游戏/环境覆盖语义
// 与 List 一致）下 distinct actor + 留痕条数，供过滤下拉消费。
func (s *Service) OperatorOptions(ctx context.Context, req *ListRequest) (*OperatorOptionsResponse, error) {
	if req == nil {
		req = &ListRequest{}
	}
	rows, err := model.NewTaskRunModel(s.svcCtx.DB).OperatorOptions(ctx, model.ListTasksOptions{
		FunctionID: req.FunctionID,
		Status:     req.Status,
		GameID:     svc.ResolveGameID(ctx, req.GameID),
		Env:        svc.ResolveEnv(ctx, req.Env),
	})
	if err != nil {
		return nil, err
	}
	items := make([]OperatorOptionsItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, OperatorOptionsItem{
			Value: row.Name,
			Label: row.Name,
			Count: row.Count,
		})
	}
	return &OperatorOptionsResponse{Items: items}, nil
}

func (s *Service) Start(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.FunctionID)
	if err != nil {
		return nil, err
	}
	scope, err := currentTaskScope(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.runtime.FindFunctionContract(ctx, scope.GameID, scope.Env, functionID); err != nil {
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

	// 统一治理闸门（ARCH-024）：禁用/分配/函数权限/政策审批/输入契约。
	// /tasks 不能再绕过任何函数级治理（旧实现只做 enable/assign/perm，
	// 缺政策审批与输入契约）。
	gate, err := functioninvoke.NewGovernance(s.svcCtx).Gate(ctx, functioninvoke.Subject{}, functioninvoke.GateInput{
		FunctionID: functionID,
		GameID:     scope.GameID,
		Env:        scope.Env,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}
	admin := gate.Admin

	gameID := scope.GameID
	env := scope.Env
	if err := utils.RequireGameEnvScope(ctx, s.svcCtx, admin.ID, gate.RoleNames, gameID, env); err != nil {
		return nil, err
	}

	// 政策要求审批：创建审批请求而非直接派发（与普通 invoke 同一语义——
	// 高危函数经 /tasks 也不能绕过审批）。
	if gate.RequireApproval && s.svcCtx.ApprovalsStore != nil {
		approvalID, err := functioninvoke.CreateFunctionApproval(ctx, s.svcCtx, &functioninvoke.Request{
			ID:       functionID,
			GameID:   gameID,
			Env:      env,
			Payload:  payload,
			Mode:     "async",
			Metadata: map[string]string{"approvalTarget": "task"},
		}, payload, admin, gate.FunctionPolicy)
		if err != nil {
			return nil, err
		}
		return &StartResponse{ApprovalID: approvalID, ApprovalRequired: true, Status: tasks.StatusDispatching}, nil
	}

	metadata := map[string]string{}
	if gameID != "" {
		metadata["gameId"] = gameID
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
	if err := requireTaskScope(ctx, run); err != nil {
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
	if err := requireTaskScope(ctx, run); err != nil {
		return nil, err
	}
	events, err := s.runtime.ListEvents(ctx, taskID, req.AfterSeq)
	if err != nil {
		return nil, err
	}
	items := make([]EventItem, 0, len(events))
	var nextSeq = req.AfterSeq
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
	run, err := s.runtime.GetRun(ctx, taskID)
	if err != nil {
		return err
	}
	if err := requireTaskScope(ctx, run); err != nil {
		return err
	}
	now := time.Now()
	updated, err := s.runtime.UpdateRunIfStatusNotIn(ctx, taskID, tasks.TerminalStatuses(), map[string]interface{}{
		"status":              tasks.StatusCancelRequested,
		"message":             "已请求取消任务",
		"cancel_requested_at": &now,
	})
	if err != nil {
		return err
	}
	if !updated {
		return errorx.NewConflictWithCode("task_terminal", "任务已结束，不能取消", nil)
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

func currentTaskScope(ctx context.Context) (svc.GameScope, error) {
	scope, err := svc.CurrentScope(ctx)
	if err != nil {
		return svc.GameScope{}, errorx.NewBadRequest("游戏环境 scope 缺失")
	}
	return scope, nil
}

func requireTaskScope(ctx context.Context, run *model.TaskRun) error {
	scope := svc.GameScopeFromContext(ctx)
	if run == nil || ((scope.GameID != "" || scope.Env != "") && !svc.ScopeMatches(ctx, run.GameID, run.Env)) {
		// Do not echo the target task's own scope into details: a caller without
		// access must not learn the taskId/gameId/env of an unauthorized resource.
		// The caller's own scope is already available in context.
		return utils.NewPermissionDenied("无权访问该任务（当前游戏/环境与任务作用域不一致）", nil)
	}
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
