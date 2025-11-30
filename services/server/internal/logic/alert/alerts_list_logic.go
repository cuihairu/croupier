// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package alert

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取告警列表
func NewAlertsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertsListLogic {
	return &AlertsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertsListLogic) AlertsList(req *types.AlertsListRequest) (resp *types.AlertsListResponse, err error) {
	if l.svcCtx.AlertModel == nil {
		return nil, errors.New("告警模型未初始化")
	}
	if req == nil {
		req = &types.AlertsListRequest{}
	}

	opts := model.ListAlertsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Level:  strings.TrimSpace(req.Level),
		Status: strings.TrimSpace(req.Status),
	}

	alerts, total, err := l.svcCtx.AlertModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Alert, 0, len(alerts))
	for i := range alerts {
		alert := alerts[i]
		var details interface{}
		if alert.Details != nil {
			details = map[string]interface{}(alert.Details)
		}
		items = append(items, types.Alert{
			Id:        alert.AlertID,
			Type:      alert.Type,
			Level:     alert.Level,
			Message:   alert.Message,
			Source:    alert.Source,
			Status:    alert.Status,
			Details:   details,
			CreatedAt: utils.FormatTimestamp(alert.CreatedAt),
		})
	}

	return &types.AlertsListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
