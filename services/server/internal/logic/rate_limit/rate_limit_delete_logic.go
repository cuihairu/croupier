// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除限流规则
func NewRateLimitDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitDeleteLogic {
	return &RateLimitDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitDeleteLogic) RateLimitDelete(req *types.RateLimitDeleteRequest) error {
	id, err := parseRateLimitID(req.ID)
	if err != nil {
		return err
	}

	if err := l.svcCtx.RateLimitModel.DeleteByKey(l.ctx, id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return fmt.Errorf("限流规则不存在")
		}
		return err
	}

	return nil
}
