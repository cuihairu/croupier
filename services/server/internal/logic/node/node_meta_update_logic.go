// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeMetaUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新节点元数据
func NewNodeMetaUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeMetaUpdateLogic {
	return &NodeMetaUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeMetaUpdateLogic) NodeMetaUpdate(req *types.NodeMetaUpdateRequest) (resp *types.NodeMetaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
