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

type OpsMQLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取消息队列状态
func NewOpsMQLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMQLogic {
	return &OpsMQLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMQLogic) OpsMQ(req *types.OpsMQRequest) (*types.OpsMQResponse, error) {
	state := snapshotOpsState(l.svcCtx)
	return &types.OpsMQResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"type":      state.MQ.Type,
			"redis":     state.MQ.Redis,
			"kafka":     state.MQ.Kafka,
			"lengths":   state.MQ.Lengths,
			"groups":    state.MQ.Groups,
			"updatedAt": utils.FormatTimestamp(state.MQ.UpdatedAt),
		},
	}, nil
}
