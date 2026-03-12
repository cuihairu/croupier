// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentExecCommandLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 在 Agent 上执行命令（高风险）
func NewOpsAgentExecCommandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentExecCommandLogic {
	return &OpsAgentExecCommandLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentExecCommandLogic) OpsAgentExecCommand(req *types.OpsExecCommandRequest) (resp *types.OpsExecCommandResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
