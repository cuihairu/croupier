package term

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type TermUpsertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type TermUpsertRequest struct {
	Domain    string `json:"domain"`
	TermKey   string `json:"term_key"`
	Alias     string `json:"alias"`
	DisplayZh string `json:"display_zh,optional"`
	DisplayEn string `json:"display_en,optional"`
	Order     int    `json:"order,optional"`
}

func NewTermUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermUpsertLogic {
	return &TermUpsertLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermUpsertLogic) TermUpsert(req *TermUpsertRequest) (map[string]interface{}, error) {
	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	termKey := strings.TrimSpace(strings.ToLower(req.TermKey))
	alias := strings.TrimSpace(strings.ToLower(req.Alias))
	if domain == "" || termKey == "" || alias == "" {
		return nil, errorx.NewBadRequest("domain/term_key/alias are required")
	}
	if domain != "entity" && domain != "operation" {
		return nil, errorx.NewBadRequest("domain must be entity or operation")
	}
	entry := &model.TermDictionary{
		Domain:    domain,
		TermKey:   termKey,
		Alias:     alias,
		DisplayZh: strings.TrimSpace(req.DisplayZh),
		DisplayEn: strings.TrimSpace(req.DisplayEn),
		SortOrder: req.Order,
	}
	if entry.SortOrder <= 0 {
		entry.SortOrder = 100
	}
	if err := l.svcCtx.TermDictModel.Upsert(l.ctx, entry); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}
