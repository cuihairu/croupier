// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取限流规则
func NewRateLimitGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitGetLogic {
	return &RateLimitGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitGetLogic) RateLimitGet(req *types.RateLimitGetRequest) (resp *types.RateLimitResponse, err error) {
	id, err := parseRateLimitID(req.ID)
	if err != nil {
		return nil, err
	}

	limit, err := l.svcCtx.RateLimitModel.FindByKey(l.ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.NewNotFound("限流规则不存在")
		}
		return nil, err
	}

	return &types.RateLimitResponse{
		RateLimit: buildRateLimitResponse(limit),
	}, nil
}
