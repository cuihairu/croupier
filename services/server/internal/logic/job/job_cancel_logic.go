// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"

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

func (l *JobCancelLogic) JobCancel(req *types.JobCancelRequest) (resp *types.JobCancelResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
