// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsSilencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取静默规则列表
func NewOpsSilencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilencesLogic {
	return &OpsSilencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilencesLogic) OpsSilences(req *types.OpsSilencesRequest) (resp *types.OpsSilencesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
