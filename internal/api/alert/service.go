package alert

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of alerts
func (s *Service) List(ctx context.Context, req *AlertsListRequest) (*AlertsListResponse, error) {
	if s.svcCtx.AlertModel == nil {
		return nil, errors.New("告警模型未初始化")
	}
	if req == nil {
		req = &AlertsListRequest{}
	}

	opts := model.ListAlertsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Level:  strings.TrimSpace(req.Level),
		Status: strings.TrimSpace(req.Status),
	}

	alerts, total, err := s.svcCtx.AlertModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Alert, 0, len(alerts))
	for i := range alerts {
		alert := alerts[i]
		var details interface{}
		if alert.Details != nil {
			details = map[string]interface{}(alert.Details)
		}
		items = append(items, Alert{
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

	return &AlertsListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Silence silences an alert
func (s *Service) Silence(ctx context.Context, req *AlertSilenceRequest) error {
	if s.svcCtx.AlertModel == nil {
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

	alertRecord, err := s.svcCtx.AlertModel.FindByAlertID(ctx, alertID)
	if err != nil {
		return err
	}

	createdBy := "system"
	if username, err := utils.CurrentUsername(ctx); err == nil && username != "" {
		createdBy = username
	}

	silence := &model.AlertSilence{
		AlertID:        alertRecord.ID,
		Reason:         strings.TrimSpace(req.Reason),
		DurationMinute: duration,
		CreatedBy:      createdBy,
	}

	return s.svcCtx.AlertModel.CreateSilence(ctx, silence)
}

// SilencesList retrieves a list of silence rules
func (s *Service) SilencesList(ctx context.Context, req *SilencesListRequest) (*SilencesListResponse, error) {
	if s.svcCtx.AlertModel == nil {
		return nil, errors.New("告警模型未初始化")
	}

	silences, err := s.svcCtx.AlertModel.ListSilences(ctx, model.ListSilencesOptions{})
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

	alertMap, err := s.svcCtx.AlertModel.FindByIDs(ctx, alertIDs)
	if err != nil {
		return nil, err
	}

	items := make([]Silence, 0, len(silences))
	for _, silence := range silences {
		var alertType string
		if alert := alertMap[silence.AlertID]; alert != nil {
			alertType = alert.Type
		}
		items = append(items, Silence{
			Id:        strconv.FormatUint(uint64(silence.ID), 10),
			AlertType: alertType,
			Matchers:  map[string]interface{}{},
			StartAt:   utils.FormatTimestamp(silence.CreatedAt),
			EndAt:     utils.FormatTimestamp(silence.ExpiresAt),
			CreatedBy: strings.TrimSpace(silence.CreatedBy),
		})
	}

	return &SilencesListResponse{
		Items: items,
	}, nil
}

// SilenceDelete deletes a silence rule
func (s *Service) SilenceDelete(ctx context.Context, req *SilenceDeleteRequest) error {
	if s.svcCtx.AlertModel == nil {
		return errors.New("告警模型未初始化")
	}
	if req == nil {
		return errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return errors.New("静默ID格式不正确")
	}

	return s.svcCtx.AlertModel.DeleteSilence(ctx, uint(id))
}
