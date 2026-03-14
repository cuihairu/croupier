package functioncall

import (
	"context"
	"strings"

	jobapi "github.com/cuihairu/croupier/internal/api/job"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
	jobSvc *jobapi.Service
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx: svcCtx,
		jobSvc: jobapi.NewService(svcCtx),
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

	jobs, err := s.jobSvc.List(ctx, &jobapi.JobListRequest{
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

	items := make([]Item, 0, len(jobs.Jobs))
	for _, job := range jobs.Jobs {
		items = append(items, fromJob(job))
	}
	return &ListResponse{
		Calls:    items,
		Total:    jobs.Total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) Detail(ctx context.Context, req *DetailRequest) (*Item, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}
	result, err := s.jobSvc.Result(ctx, &jobapi.JobResultRequest{ID: jobID})
	if err != nil {
		return nil, err
	}
	status := "running"
	if result.Done {
		status = "completed"
	}
	return &Item{
		ID:        jobID,
		JobID:     jobID,
		Status:    status,
		Result:    result.Events,
		CreatedAt: "",
	}, nil
}

func (s *Service) Cancel(ctx context.Context, req *DetailRequest) error {
	return s.jobSvc.Cancel(ctx, &jobapi.JobCancelRequest{ID: strings.TrimSpace(req.ID)})
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

func fromJob(job jobapi.JobItem) Item {
	status := job.State
	if status == "" {
		status = "unknown"
	}
	return Item{
		ID:         job.ID,
		JobID:      job.ID,
		FunctionID: job.FunctionID,
		GameID:     job.GameID,
		Env:        job.Env,
		Status:     status,
		StartedAt:  job.StartedAt,
		FinishedAt: job.EndedAt,
		DurationMs: job.DurationMs,
		ErrorMsg:   job.Error,
		CreatedAt:  job.StartedAt,
	}
}
