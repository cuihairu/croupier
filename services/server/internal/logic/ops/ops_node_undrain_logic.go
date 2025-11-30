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

type OpsNodeUndrainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消排空节点
func NewOpsNodeUndrainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeUndrainLogic {
	return &OpsNodeUndrainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeUndrainLogic) OpsNodeUndrain(req *types.OpsNodeActionRequest) (*types.OpsNodeUndrainResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.NodeID)
	if err != nil {
		return nil, err
	}

	actionReq := &types.NodeActionRequest{ID: nodeID}
	if err := nodelogic.NewNodeUndrainLogic(l.ctx, l.svcCtx).NodeUndrain(actionReq); err != nil {
		return nil, err
	}

	return &types.OpsNodeUndrainResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"nodeId": nodeID,
			"status": "active",
		},
	}, nil
}
