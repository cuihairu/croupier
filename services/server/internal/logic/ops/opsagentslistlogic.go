// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 列表
func NewOpsAgentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentsListLogic {
	return &OpsAgentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentsListLogic) OpsAgentsList(req *types.OpsAgentsListRequest) (resp *types.OpsAgentsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
