// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动任务
func NewJobStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobStartLogic {
	return &JobStartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobStartLogic) JobStart(req *types.JobStartRequest) (resp *types.JobStartResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
