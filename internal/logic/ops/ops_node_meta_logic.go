package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsNodeMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点元数据
func NewOpsNodeMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsNodeMetaLogic {
	return &OpsNodeMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsNodeMetaLogic) OpsNodeMeta(req *OpsNodeMetaRequest) (resp *OpsNodeMetaResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsNodeMeta not implemented")
}
