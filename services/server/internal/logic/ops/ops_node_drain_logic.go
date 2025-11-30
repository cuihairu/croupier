// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	nodelogic "github.com/cuihairu/croupier/services/server/internal/logic/node"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeDrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 排空节点
func NewOpsNodeDrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeDrainLogic {
	return &OpsNodeDrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeDrainLogic) OpsNodeDrain(req *types.OpsNodeActionRequest) (*types.OpsNodeDrainResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.NodeID)
	if err != nil {
		return nil, err
	}

	nodeReq := &types.NodeDrainRequest{ID: nodeID}
	if err := nodelogic.NewNodeDrainLogic(l.ctx, l.svcCtx).NodeDrain(nodeReq); err != nil {
		return nil, err
	}

	return &types.OpsNodeDrainResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"nodeId": nodeID,
			"status": "draining",
		},
	}, nil
}
