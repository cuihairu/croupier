package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsAlerts(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertsRequest) (*OpsAlertsResponse, error) {
	// 只输出基础设施告警；业务告警（如 schema 破坏性变更）由函数域诊断呈现
	// 4862：按请求 scope 过滤（当前 scope 告警 + 平台级告警）。
	scope := svc.GameScopeFromContext(ctx)
	opts := model.ListAlertsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
		ExcludeSources:    []string{alertSourceContract},
		ScopeGameID:       scope.GameID,
		ScopeEnv:          scope.Env,
	}
	if req != nil {
		opts.HandleStatus = strings.TrimSpace(req.HandleStatus)
	}
	alerts, _, err := svcCtx.AlertModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]OpsAlert, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, OpsAlert{
			Severity:      a.Level,
			Service:       a.Type,
			Summary:       a.Message,
			StartsAt:      utils.FormatTimestamp(a.CreatedAt),
			Labels:        map[string]interface{}{"id": fmt.Sprintf("%d", a.ID)},
			Annotations:   map[string]interface{}{"status": a.Status},
			HandleStatus:  a.HandleStatus,
			HandledBy:     a.HandledBy,
			HandledRemark: a.HandledRemark,
			HandledAt:     utils.FormatTimestampPtr(a.HandledAt),
		})
	}

	return &OpsAlertsResponse{
		Alerts: items,
	}, nil
}

// alertHandleActions 工作流动作 → 状态（风险事件扩展 v1：流转从简，任意
// 合法动作可流转到不同目标态，审计字段足够追责；高危流转（升级/封号）的
// 审批钩子留待后续批次）。
var alertHandleActions = map[string]string{
	"acknowledge": model.AlertHandleAcknowledged,
	"ignore":      model.AlertHandleIgnored,
	"escalate":    model.AlertHandleEscalated,
}

func opsAlertHandle(ctx context.Context, svcCtx *svc.ServiceContext, alertIDStr string, req *OpsAlertHandleRequest) (*OpsAlertHandleResponse, error) {
	if req == nil {
		return nil, errorx.NewValidationError("request is required")
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	target, ok := alertHandleActions[action]
	if !ok {
		return nil, errorx.NewValidationError("action must be one of: acknowledge, ignore, escalate")
	}

	var alertID uint
	if _, err := fmt.Sscanf(strings.TrimSpace(alertIDStr), "%d", &alertID); err != nil || alertID == 0 {
		return nil, errorx.NewBadRequest("invalid alert id")
	}

	// 存在性检查：流转不隐式创建，404 而非静默成功。
	foundAlerts, err := svcCtx.AlertModel.FindByIDs(ctx, []uint{alertID})
	if err != nil {
		return nil, err
	}
	storedAlert, ok := foundAlerts[alertID]
	if !ok {
		return nil, errorx.NewNotFound(fmt.Sprintf("alert %d not found", alertID))
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && storedAlert.GameID != scope.GameID {
		return nil, fmt.Errorf("alert %d does not belong to game %q", alertID, scope.GameID)
	}
	if scope.Env != "" && storedAlert.Env != scope.Env {
		return nil, fmt.Errorf("alert %d does not belong to env %q", alertID, scope.Env)
	}

	remark := strings.TrimSpace(req.Remark)
	if err := svcCtx.AlertModel.UpdateHandling(ctx, alertID, model.HandleTransition{
		HandleStatus:  target,
		HandledBy:     captureGatesAuthor(ctx),
		HandledRemark: remark,
	}); err != nil {
		return nil, err
	}

	return &OpsAlertHandleResponse{
		AlertID:      fmt.Sprintf("%d", alertID),
		HandleStatus: target,
		HandledBy:    captureGatesAuthor(ctx),
		HandledAt:    utils.FormatTimestamp(time.Now()),
	}, nil
}

func opsAlertSilence(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertSilenceRequest) (*OpsAlertSilenceResponse, error) {
	// Parse alertID to uint
	var alertID uint
	if _, err := fmt.Sscanf(req.AlertID, "%d", &alertID); err != nil {
		return nil, err
	}
	// Create a silence for this alert
	silence := &model.AlertSilence{
		AlertID:        alertID,
		DurationMinute: req.Duration,
	}
	if err := svcCtx.AlertModel.CreateSilence(ctx, silence); err != nil {
		return nil, err
	}

	return &OpsAlertSilenceResponse{
		SilenceID: fmt.Sprintf("%d", silence.ID),
	}, nil
}

func opsSilenceDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertSilenceRequest) (*OpsSilenceDeleteResponse, error) {
	// Parse the silence ID as uint
	var silenceID uint
	if _, err := fmt.Sscanf(req.AlertID, "%d", &silenceID); err != nil {
		return nil, errorx.NewBadRequest("Invalid silence ID")
	}
	if err := svcCtx.AlertModel.DeleteSilence(ctx, silenceID); err != nil {
		return nil, err
	}

	return &OpsSilenceDeleteResponse{
		Deleted: true,
	}, nil
}

func opsSilences(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsSilencesRequest) (*OpsSilencesResponse, error) {
	silences, err := svcCtx.AlertModel.ListSilences(ctx, model.ListSilencesOptions{})
	if err != nil {
		return nil, err
	}

	items := make([]Silence, 0, len(silences))
	for _, s := range silences {
		items = append(items, Silence{
			Id:        fmt.Sprintf("%d", s.ID),
			AlertType: fmt.Sprintf("%d", s.AlertID),
			StartAt:   utils.FormatTimestamp(s.CreatedAt),
			EndAt:     utils.FormatTimestamp(s.ExpiresAt),
			CreatedBy: s.CreatedBy,
		})
	}

	return &OpsSilencesResponse{
		Silences: items,
	}, nil
}

// Node operations implementations
