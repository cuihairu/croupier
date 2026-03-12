// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodeMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点元数据
func NewNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeMetaLogic {
	return &NodeMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeMetaLogic) NodeMeta(req *types.NodeMetaRequest) (resp *types.NodeMetaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
