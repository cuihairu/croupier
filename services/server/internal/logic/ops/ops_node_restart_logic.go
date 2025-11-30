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

type OpsNodeRestartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启节点
func NewOpsNodeRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeRestartLogic {
	return &OpsNodeRestartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeRestartLogic) OpsNodeRestart(req *types.OpsNodeActionRequest) (*types.OpsNodeRestartResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.NodeID)
	if err != nil {
		return nil, err
	}

	actionReq := &types.NodeActionRequest{ID: nodeID}
	if err := nodelogic.NewNodeRestartLogic(l.ctx, l.svcCtx).NodeRestart(actionReq); err != nil {
		return nil, err
	}

	return &types.OpsNodeRestartResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"nodeId": nodeID,
			"status": "restarting",
		},
	}, nil
}
