// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点列表
func NewOpsNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodesLogic {
	return &OpsNodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodesLogic) OpsNodes(req *types.OpsNodesRequest) (resp *types.OpsNodesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
