// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览限流规则
func NewRateLimitPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitPreviewLogic {
	return &RateLimitPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitPreviewLogic) RateLimitPreview(req *types.RateLimitPreviewRequest) (resp *types.RateLimitPreviewResponse, err error) {
	rulesMap, err := normalizeRules(req.Rules)
	if err != nil {
		return nil, err
	}
	if len(rulesMap) == 0 {
		return nil, errors.New("请提供要预览的规则条件")
	}

	keys := summarizeRuleKeys(rulesMap)
	complexity := classifyComplexity(len(keys))

	matches := map[string]interface{}{
		"rules":          rulesMap,
		"matchedFields":  keys,
		"sampleEntities": []string{"player-001", "player-207"},
	}
	impact := map[string]interface{}{
		"complexity": complexity,
		"notes":      "预估结果仅供调试参考，真实流量需结合监控确认。",
	}

	return &types.RateLimitPreviewResponse{
		Matches: matches,
		Impact:  impact,
	}, nil
}
