
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsAgentMetaLogic) OpsAgentMeta(req *OpsAgentMetaUpdateRequest) (resp *OpsAgentMetaResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsAgentMeta not implemented")
}
