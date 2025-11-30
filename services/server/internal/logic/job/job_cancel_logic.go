// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobCancelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消任务
func NewJobCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobCancelLogic {
	return &JobCancelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobCancelLogic) JobCancel(req *types.JobCancelRequest) (*types.JobCancelResponse, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}

	if err := l.svcCtx.Dispatcher.CancelJob(l.ctx, jobID); err != nil {
		return nil, err
	}

	return &types.JobCancelResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
