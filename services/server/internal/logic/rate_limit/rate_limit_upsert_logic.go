// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"
	"errors"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type RateLimitUpsertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建/更新限流规则
func NewRateLimitUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitUpsertLogic {
	return &RateLimitUpsertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitUpsertLogic) RateLimitUpsert(req *types.RateLimitUpsertRequest) (resp *types.RateLimitResponse, err error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("限流规则名称不能为空")
	}

	resource := strings.TrimSpace(req.Resource)
	if resource == "" {
		return nil, errors.New("资源类型不能为空")
	}

	if req.Limit <= 0 {
		return nil, errors.New("Limit 必须大于0")
	}
	if req.Window <= 0 {
		return nil, errors.New("Window 必须大于0")
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "reject", "throttle":
	default:
		return nil, errorx.NewBadRequest("Action 无效，只能是 reject 或 throttle")
	}

	rulesMap, err := normalizeRules(req.Rules)
	if err != nil {
		return nil, err
	}

	limit := &model.RateLimit{
		RateLimitID: generateRateLimitID(resource, name),
		Name:        name,
		Resource:    resource,
		Limit:       req.Limit,
		Window:      req.Window,
		Action:      action,
		Rules:       datatypes.JSONMap{},
		Status:      1,
	}
	if rulesMap != nil {
		limit.Rules = encodeRules(rulesMap)
	}

	if err := l.svcCtx.RateLimitModel.Upsert(l.ctx, limit); err != nil {
		return nil, errorx.NewInternalError("保存限流规则失败")
	}

	updated, err := l.svcCtx.RateLimitModel.FindByKey(l.ctx, limit.RateLimitID)
	if err != nil {
		return nil, err
	}

	return &types.RateLimitResponse{
		RateLimit: buildRateLimitResponse(updated),
	}, nil
}
