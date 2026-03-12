// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_overview

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RealtimeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实时数据
func NewRealtimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RealtimeLogic {
	return &RealtimeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RealtimeLogic) Realtime(req *types.RealtimeRequest) (resp *types.RealtimeResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
