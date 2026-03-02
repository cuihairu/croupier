package term

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type TermDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type TermDeleteRequest struct {
	Domain string `form:"domain"`
	Alias  string `form:"alias"`
}

func NewTermDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TermDeleteLogic {
	return &TermDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TermDeleteLogic) TermDelete(req *TermDeleteRequest) (map[string]interface{}, error) {
	domain := strings.TrimSpace(strings.ToLower(req.Domain))
	alias := strings.TrimSpace(strings.ToLower(req.Alias))
	if domain == "" || alias == "" {
		return nil, errorx.NewBadRequest("domain and alias are required")
	}
	if err := l.svcCtx.TermDictModel.DeleteByAlias(l.ctx, domain, alias); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}
