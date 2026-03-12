// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentProcessesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 进程列表
func NewOpsAgentProcessesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessesLogic {
	return &OpsAgentProcessesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessesLogic) OpsAgentProcesses(req *types.OpsAgentProcessesRequest) (resp *types.OpsAgentProcessesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
