// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type CertificateAlertsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书告警列表
func NewCertificateAlertsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertsListLogic {
	return &CertificateAlertsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAlertsListLogic) CertificateAlertsList(req *types.CertificateAlertsListRequest) (*types.CertificateAlertsListResponse, error) {
	alerts, total, err := l.svcCtx.CertificateModel.ListAlerts(l.ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(alerts))
	for _, alert := range alerts {
		items = append(items, map[string]interface{}{
			"id":              alert.ID,
			"domain":          alert.Domain,
			"thresholdDays":   alert.ThresholdDays,
			"active":          alert.Active,
			"lastTriggeredAt": utils.FormatTimestampPtr(alert.LastTriggeredAt),
			"createdAt":       utils.FormatTimestamp(alert.CreatedAt),
		})
	}

	return &types.CertificateAlertsListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  req.Page,
			"size":  req.PageSize,
		},
	}, nil
}
