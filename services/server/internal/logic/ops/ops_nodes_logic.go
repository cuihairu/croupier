// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
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

func (l *OpsNodesLogic) OpsNodes(req *types.OpsNodesRequest) (*types.OpsNodesResponse, error) {
	nodes := make([]map[string]interface{}, 0)
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
				nodes = append(nodes, snapshot)
			}
		}
		store.Mu().RUnlock()
	}

	return &types.OpsNodesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"nodes": nodes,
		},
	}, nil
}
