// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAlertSilenceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 静默告警
func NewOpsAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertSilenceLogic {
	return &OpsAlertSilenceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertSilenceLogic) OpsAlertSilence(req *types.OpsAlertSilenceRequest) (*types.OpsAlertSilenceResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	alertID := strings.TrimSpace(req.AlertID)
	if alertID == "" {
		return nil, errors.New("告警ID不能为空")
	}
	duration := req.Duration
	if duration <= 0 {
		duration = 60
	}

	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		silence := buildSilence(alertID, duration)
		st.Alerts.Silences = append([]svc.OpsSilenceEntry{silence}, st.Alerts.Silences...)
		st.Alerts.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, err
	}

	silence := state.Alerts.Silences[0]
	return &types.OpsAlertSilenceResponse{
		Code:    0,
		Message: "OK",
		Data:    serializeSilence(silence),
	}, nil
}

func buildSilence(alertID string, durationMinutes int) svc.OpsSilenceEntry {
	now := time.Now().UTC()
	id := fmt.Sprintf("sil-%s", strings.ReplaceAll(uuid.New().String(), "-", ""))
	return svc.OpsSilenceEntry{
		ID:        id,
		AlertID:   alertID,
		CreatedBy: "ops",
		StartsAt:  now,
		EndsAt:    now.Add(time.Duration(durationMinutes) * time.Minute),
		Status: svc.OpsSilenceStatus{
			State: "active",
		},
	}
}

func serializeSilence(entry svc.OpsSilenceEntry) map[string]interface{} {
	return map[string]interface{}{
		"id":         entry.ID,
		"alertId":    entry.AlertID,
		"created_by": entry.CreatedBy,
		"starts_at":  utils.FormatTimestamp(entry.StartsAt),
		"ends_at":    utils.FormatTimestamp(entry.EndsAt),
		"status": map[string]interface{}{
			"state": entry.Status.State,
		},
		"matchers": entry.Matchers,
		"comment":  entry.Comment,
	}
}
