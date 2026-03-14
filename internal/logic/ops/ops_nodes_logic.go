package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsNodesLogic) OpsNodes(req *OpsNodesRequest) (resp *OpsNodesResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodes not implemented")
}
