// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

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

func (l *OpsNodeUndrainLogic) OpsNodeUndrain(req *types.OpsNodeActionRequest) (resp *types.OpsNodeUndrainResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
