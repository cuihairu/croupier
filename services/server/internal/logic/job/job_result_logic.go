// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type JobResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取任务结果
func NewJobResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobResultLogic {
	return &JobResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobResultLogic) JobResult(req *types.JobResultRequest) (*types.JobResultResponse, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}

	events, done, err := l.svcCtx.Dispatcher.StreamJob(l.ctx, jobID)
	if err != nil {
		return nil, err
	}

	return &types.JobResultResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"jobId":  jobID,
			"done":   done,
			"events": events,
		},
	}, nil
}
