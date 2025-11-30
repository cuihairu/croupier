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

type OpsNodeMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点元数据
func NewOpsNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeMetaLogic {
	return &OpsNodeMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeMetaLogic) OpsNodeMeta(req *types.OpsNodeMetaRequest) (*types.OpsNodeMetaResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.NodeID)
	if err != nil {
		return nil, err
	}

	node, err := l.svcCtx.NodeModel.FindByNodeID(l.ctx, nodeID)
	if err != nil {
		return nil, err
	}

	nodeData := map[string]interface{}{
		"id":        node.NodeID,
		"name":      node.Name,
		"type":      node.Type,
		"status":    node.Status,
		"ip":        node.IP,
		"port":      node.Port,
		"resources": node.Resources,
		"meta":      node.Meta,
		"updatedAt": utils.FormatTimestamp(node.UpdatedAt),
	}

	data := map[string]interface{}{
		"node": nodeData,
	}

	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		if snapshot := utils.BuildOpsAgentSnapshot(store.AgentsUnsafe()[nodeID]); snapshot != nil {
			data["runtime"] = snapshot
		}
		store.Mu().RUnlock()
	}

	return &types.OpsNodeMetaResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
