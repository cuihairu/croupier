package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type JobStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJobStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobStatusLogic {
	return &JobStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobStatusLogic) JobStatus(req *types.JobStatusRequest) (resp *types.JobStatusResponse, err error) {
	logx.Infof("Job status request: jobId=%s", req.JobId)

	// Get job from manager
	job, exists := l.svcCtx.JobManager.GetJob(req.JobId)
	if !exists {
		return &types.JobStatusResponse{
			JobId:  req.JobId,
			Status: "not_found",
			Error:  "Job not found",
		}, nil
	}

	return &types.JobStatusResponse{
		JobId:    job.ID,
		Status:   job.Status,
		Result:   job.Result,
		Error:    job.Error,
		Progress: job.Progress,
		StartTime: job.StartTime,
		EndTime:  job.EndTime,
	}, nil
}