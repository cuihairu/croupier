// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"strings"

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

func (l *JobStatusLogic) JobStatus(req *types.JobStatusRequest) (*types.JobStatusResponse, error) {
	if req == nil || strings.TrimSpace(req.JobId) == "" {
		return nil, errors.New("job_id 不能为空")
	}

	state := l.svcCtx.AgentState
	state.Mu.RLock()
	record := state.Jobs[req.JobId]
	state.Mu.RUnlock()

	if record == nil {
		return nil, errors.New("job not found")
	}

	var end string
	if record.EndTime != nil {
		end = formatTime(*record.EndTime)
	}

	return &types.JobStatusResponse{
		JobId:     record.ID,
		Status:    record.Status,
		Result:    record.Result,
		Error:     record.Error,
		Progress:  100,
		StartTime: formatTime(record.StartTime),
		EndTime:   end,
	}, nil
}
