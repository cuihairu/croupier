// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SilencesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取静默规则列表
func NewSilencesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SilencesListLogic {
	return &SilencesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SilencesListLogic) SilencesList(req *types.SilencesListRequest) (resp *types.SilencesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
