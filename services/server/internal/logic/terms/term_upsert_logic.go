// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package terms

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type TermUpsertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建/更新术语
func NewTermUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermUpsertLogic {
	return &TermUpsertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermUpsertLogic) TermUpsert(req *types.TermUpsertRequest) (resp *types.TermUpsertResponse, err error) {
	// 创建或更新术语
	term := &model.TermDictionary{
		Domain:    req.Domain,
		TermKey:   req.TermKey,
		Alias:     req.Alias,
		DisplayZh: req.DisplayZh,
		DisplayEn: req.DisplayEn,
		SortOrder: int(req.Order),
	}

	err = l.svcCtx.TermDictModel.Upsert(l.ctx, term)
	if err != nil {
		return nil, err
	}

	return &types.TermUpsertResponse{
		Ok: true,
	}, nil
}
