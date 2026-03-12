// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package job

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamJobLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 任务流（实时状态和日志）
func NewStreamJobLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamJobLogic {
	return &StreamJobLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StreamJobLogic) StreamJob(req *types.StreamJobRequest) (resp *types.StreamJobResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
