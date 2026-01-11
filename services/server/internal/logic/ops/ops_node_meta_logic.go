// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsNodeMetaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点元数据
func NewOpsNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeMetaLogic {
	return &OpsNodeMetaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeMetaLogic) OpsNodeMeta(req *types.OpsNodeMetaRequest) (resp *types.OpsNodeMetaResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
