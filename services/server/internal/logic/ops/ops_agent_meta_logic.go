// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsAgentMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新代理元数据
func NewOpsAgentMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentMetaLogic {
	return &OpsAgentMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentMetaLogic) OpsAgentMeta(req *types.OpsAgentMetaUpdateRequest) (resp *types.OpsAgentMetaResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAgentMeta not implemented")
}
