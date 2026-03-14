// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package terms

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TermDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除术语
func NewTermDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermDeleteLogic {
	return &TermDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermDeleteLogic) TermDelete(req *types.TermDeleteRequest) (resp *types.TermDeleteResponse, err error) {
	// 删除术语
	err = l.svcCtx.TermDictModel.DeleteByAlias(l.ctx, req.Domain, req.Alias)
	if err != nil {
		return nil, err
	}

	return &types.TermDeleteResponse{
		Ok: true,
	}, nil
}
