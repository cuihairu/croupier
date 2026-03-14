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

type JobCancelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消任务
func NewJobCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobCancelLogic {
	return &JobCancelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobCancelLogic) JobCancel(req *types.JobCancelRequest) (*types.JobCancelResponse, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}

	if err := l.svcCtx.Dispatcher.CancelJob(l.ctx, jobID); err != nil {
		return nil, err
	}

	return &types.JobCancelResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
