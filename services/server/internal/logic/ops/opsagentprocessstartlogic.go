// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentProcessStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动 Agent 进程
func NewOpsAgentProcessStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStartLogic {
	return &OpsAgentProcessStartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStartLogic) OpsAgentProcessStart(req *types.OpsProcessStartRequest) (resp *types.OpsProcessStartResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
