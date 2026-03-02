package term

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type TermsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type TermsListRequest struct {
	Domain string `form:"domain,optional"`
}

func NewTermsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermsListLogic {
	return &TermsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermsListLogic) TermsList(req *TermsListRequest) (map[string]interface{}, error) {
	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	items, err := l.svcCtx.TermDictModel.List(l.ctx, domain)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]interface{}{
			"id":         it.ID,
			"domain":     it.Domain,
			"term_key":   it.TermKey,
			"alias":      it.Alias,
			"display_zh": it.DisplayZh,
			"display_en": it.DisplayEn,
			"order":      it.SortOrder,
		})
	}
	return map[string]interface{}{"items": out}, nil
}
