// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type JobExecuteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJobExecuteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobExecuteLogic {
	return &JobExecuteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobExecuteLogic) JobExecute(req *types.JobExecuteRequest) (resp *types.JobExecuteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
