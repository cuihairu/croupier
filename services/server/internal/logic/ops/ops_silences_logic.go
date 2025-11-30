// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsSilencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取静默规则列表
func NewOpsSilencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilencesLogic {
	return &OpsSilencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilencesLogic) OpsSilences(req *types.OpsSilencesRequest) (*types.OpsSilencesResponse, error) {
	state := snapshotOpsState(l.svcCtx)
	items := make([]map[string]interface{}, 0, len(state.Alerts.Silences))
	for _, silence := range state.Alerts.Silences {
		items = append(items, map[string]interface{}{
			"id":         silence.ID,
			"alert_id":   silence.AlertID,
			"created_by": silence.CreatedBy,
			"starts_at":  utils.FormatTimestamp(silence.StartsAt),
			"ends_at":    utils.FormatTimestamp(silence.EndsAt),
			"status": map[string]interface{}{
				"state": silence.Status.State,
			},
			"matchers": silence.Matchers,
			"comment":  silence.Comment,
		})
	}

	return &types.OpsSilencesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"silences": items,
		},
	}, nil
}
