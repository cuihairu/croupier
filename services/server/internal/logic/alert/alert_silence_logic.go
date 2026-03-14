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
)

type AlertSilenceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 静默告警
func NewAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertSilenceLogic {
	return &AlertSilenceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertSilenceLogic) AlertSilence(req *types.AlertSilenceRequest) error {
	if l.svcCtx.AlertModel == nil {
		return errors.New("告警模型未初始化")
	}
	if req == nil {
		return errors.New("请求体不能为空")
	}

	alertID := strings.TrimSpace(req.ID)
	if alertID == "" {
		return errors.New("告警ID不能为空")
	}
	duration := req.Duration
	if duration <= 0 {
		duration = 60
	}

	alertRecord, err := l.svcCtx.AlertModel.FindByAlertID(l.ctx, alertID)
	if err != nil {
		return err
	}

	createdBy := "system"
	if username, err := utils.CurrentUsername(l.ctx); err == nil && username != "" {
		createdBy = username
	}

	silence := &model.AlertSilence{
		AlertID:        alertRecord.ID,
		Reason:         strings.TrimSpace(req.Reason),
		DurationMinute: duration,
		CreatedBy:      createdBy,
	}

	return l.svcCtx.AlertModel.CreateSilence(l.ctx, silence)
}
