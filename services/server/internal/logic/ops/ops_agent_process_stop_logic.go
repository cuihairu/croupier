// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentProcessStopLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 停止 Agent 进程
func NewOpsAgentProcessStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStopLogic {
	return &OpsAgentProcessStopLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStopLogic) OpsAgentProcessStop(req *types.OpsProcessActionRequest) (resp *types.OpsProcessActionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
