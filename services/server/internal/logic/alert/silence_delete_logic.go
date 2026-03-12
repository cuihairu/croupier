// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SilenceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除静默规则
func NewSilenceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SilenceDeleteLogic {
	return &SilenceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SilenceDeleteLogic) SilenceDelete(req *types.SilenceDeleteRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
