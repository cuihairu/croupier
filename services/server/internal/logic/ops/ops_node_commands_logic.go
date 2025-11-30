// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeCommandsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点命令
func NewOpsNodeCommandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeCommandsLogic {
	return &OpsNodeCommandsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeCommandsLogic) OpsNodeCommands(req *types.OpsNodeCommandsQuery) (*types.OpsNodeCommandsResponse, error) {
	commands, err := l.svcCtx.NodeModel.ListCommands(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.NodeCommand, 0, len(commands))
	for i := range commands {
		items = append(items, types.NodeCommand{
			Name:        commands[i].Name,
			Description: commands[i].Description,
		})
	}

	return &types.OpsNodeCommandsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"commands": items,
		},
	}, nil
}
