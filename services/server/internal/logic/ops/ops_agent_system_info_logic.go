// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentSystemInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 系统信息
func NewOpsAgentSystemInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentSystemInfoLogic {
	return &OpsAgentSystemInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentSystemInfoLogic) OpsAgentSystemInfo(req *types.OpsAgentSystemInfoRequest) (resp *types.OpsAgentSystemInfoResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
