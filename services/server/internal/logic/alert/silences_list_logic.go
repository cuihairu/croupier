// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type SilencesListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取静默规则列表
func NewSilencesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SilencesListLogic {
	return &SilencesListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SilencesListLogic) SilencesList(req *types.SilencesListRequest) (resp *types.SilencesListResponse, err error) {
	if l.svcCtx.AlertModel == nil {
		return nil, errors.New("告警模型未初始化")
	}

	silences, err := l.svcCtx.AlertModel.ListSilences(l.ctx, model.ListSilencesOptions{})
	if err != nil {
		return nil, err
	}

	alertIDs := make([]uint, 0, len(silences))
	seen := make(map[uint]struct{})
	for _, silence := range silences {
		if silence.AlertID == 0 {
			continue
		}
		if _, ok := seen[silence.AlertID]; ok {
			continue
		}
		seen[silence.AlertID] = struct{}{}
		alertIDs = append(alertIDs, silence.AlertID)
	}

	alertMap, err := l.svcCtx.AlertModel.FindByIDs(l.ctx, alertIDs)
	if err != nil {
		return nil, err
	}

	items := make([]types.Silence, 0, len(silences))
	for _, silence := range silences {
		var alertType string
		if alert := alertMap[silence.AlertID]; alert != nil {
			alertType = alert.Type
		}
		items = append(items, types.Silence{
			Id:        strconv.FormatUint(uint64(silence.ID), 10),
			AlertType: alertType,
			Matchers:  map[string]interface{}{},
			StartAt:   utils.FormatTimestamp(silence.CreatedAt),
			EndAt:     utils.FormatTimestamp(silence.ExpiresAt),
			CreatedBy: strings.TrimSpace(silence.CreatedBy),
		})
	}

	return &types.SilencesListResponse{
		Items: items,
	}, nil
}
