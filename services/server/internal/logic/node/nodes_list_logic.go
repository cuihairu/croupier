// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NodesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点列表
func NewNodesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodesListLogic {
	return &NodesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodesListLogic) NodesList(req *types.NodesListRequest) (resp *types.NodesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
