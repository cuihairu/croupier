// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsNodesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点列表
func NewOpsNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodesLogic {
	return &OpsNodesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodesLogic) OpsNodes(req *types.OpsNodesRequest) (resp *types.OpsNodesResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodes not implemented")
}
