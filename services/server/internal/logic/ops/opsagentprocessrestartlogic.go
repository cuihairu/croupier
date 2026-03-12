// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentProcessRestartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启 Agent 进程
func NewOpsAgentProcessRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessRestartLogic {
	return &OpsAgentProcessRestartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessRestartLogic) OpsAgentProcessRestart(req *types.OpsProcessActionRequest) (resp *types.OpsProcessActionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
