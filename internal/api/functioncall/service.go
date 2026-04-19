package functioncall

import (
	"context"
	"strings"

	taskapi "github.com/cuihairu/croupier/internal/api/task"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx  *svc.ServiceContext
	taskSvc *taskapi.Service
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx:  svcCtx,
		taskSvc: taskapi.NewService(svcCtx),
	}
}

func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	tasks, err := s.taskSvc.List(ctx, &taskapi.ListRequest{
		Status:     req.Status,
		FunctionID: req.FunctionID,
		GameID:     req.GameID,
		Env:        req.Env,
		Page:       page,
		Size:       pageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(tasks.Items))
	for _, task := range tasks.Items {
		items = append(items, fromTask(task))
	}
	return &ListResponse{
		Calls:    items,
		Total:    tasks.Total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) Detail(ctx context.Context, req *DetailRequest) (*Item, error) {
	taskID := strings.TrimSpace(req.ID)
	if taskID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}
	result, err := s.taskSvc.Detail(ctx, &taskapi.DetailRequest{ID: taskID})
	if err != nil {
		return nil, err
	}
	return &Item{
		ID:         taskID,
		TaskID:     taskID,
		Status:     result.Status,
		Result:     result.Result,
		ErrorMsg:   result.Error,
		CreatedAt:  result.CreatedAt,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
	}, nil
}

func (s *Service) Cancel(ctx context.Context, req *DetailRequest) error {
	return s.taskSvc.Cancel(ctx, &taskapi.CancelRequest{ID: strings.TrimSpace(req.ID)})
}

func (s *Service) Stats(ctx context.Context, req *ListRequest) (*StatsResponse, error) {
	list, err := s.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &StatsResponse{Total: list.Total}
	for _, item := range list.Calls {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "success", "succeeded", "completed":
			resp.Succeeded++
		case "failed", "error":
			resp.Failed++
		case "running", "pending":
			resp.Running++
		case "cancelled", "canceled":
			resp.Cancelled++
		case "timeout":
			resp.Timeout++
		default:
			resp.Other++
		}
	}
	return resp, nil
}

func (s *Service) Rerun(ctx context.Context, req *RerunRequest) (*RerunResponse, error) {
	return nil, errorx.NewBadRequest("当前版本暂不支持从调用历史重跑")
}

func fromTask(task taskapi.Item) Item {
	status := task.Status
	if status == "" {
		status = "unknown"
	}
	return Item{
		ID:         task.ID,
		TaskID:     task.ID,
		FunctionID: task.FunctionID,
		GameID:     task.GameID,
		Env:        task.Env,
		Status:     status,
		AgentID:    task.AgentID,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
		ErrorMsg:   task.Error,
		CreatedAt:  task.CreatedAt,
	}
}
