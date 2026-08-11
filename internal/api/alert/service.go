package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const officialAlertingID = "official.alerting"
const alertingSilencesKey = "silences"

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
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Level:             strings.TrimSpace(req.Level),
		Status:            strings.TrimSpace(req.Status),
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

	if err := s.svcCtx.AlertModel.CreateSilence(ctx, silence); err != nil {
		return err
	}
	_ = s.appendAlertingSilenceToExtension(ctx, alertRecord.Type, silence)
	_ = s.recordAlertingEvent(ctx, "alerts_silence", "alert silenced",
		fmt.Sprintf(`{"alert_id":"%s","duration":%d}`, alertID, duration),
	)
	return nil
}

// SilencesList retrieves a list of silence rules
func (s *Service) SilencesList(ctx context.Context, req *SilencesListRequest) (*SilencesListResponse, error) {
	if items, ok, err := s.loadAlertingSilencesFromExtension(ctx); err != nil {
		return nil, err
	} else if ok {
		return &SilencesListResponse{Items: items}, nil
	}
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
	if id > math.MaxUint {
		return errors.New("静默ID超出范围")
	}

	if err := s.svcCtx.AlertModel.DeleteSilence(ctx, uint(id)); err != nil {
		return err
	}
	_ = s.removeAlertingSilenceFromExtension(ctx, strconv.FormatUint(id, 10))
	_ = s.recordAlertingEvent(ctx, "alerts_unsilence", "alert silence deleted",
		fmt.Sprintf(`{"silence_id":%d}`, id),
	)
	return nil
}

func (s *Service) findActiveAlertingInstallation(ctx context.Context) (*model.ExtensionInstallation, bool, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Extensions == nil || s.svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: officialAlertingID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func (s *Service) recordAlertingEvent(ctx context.Context, eventType, message, payload string) error {
	item, ok, err := s.findActiveAlertingInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	operator := "system"
	if username, err := utils.CurrentUsername(ctx); err == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.RecordEvent(ctx, item.ID, eventType, "info", message, operator, payload)
}

func (s *Service) loadAlertingSilencesFromExtension(ctx context.Context) ([]Silence, bool, error) {
	item, ok, err := s.findActiveAlertingInstallation(ctx)
	if err != nil || !ok || item == nil {
		return nil, false, err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return nil, false, err
		}
	}
	raw, exists := config[alertingSilencesKey]
	if !exists || raw == nil {
		return nil, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	items := []Silence{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}

func (s *Service) saveAlertingSilencesToExtension(ctx context.Context, items []Silence) error {
	item, ok, err := s.findActiveAlertingInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config[alertingSilencesKey] = items
	secretRefs := map[string]string{}
	if len(bytes.TrimSpace(item.SecretRefsJSON)) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func (s *Service) appendAlertingSilenceToExtension(ctx context.Context, alertType string, silence *model.AlertSilence) error {
	if silence == nil {
		return nil
	}
	items, _, err := s.loadAlertingSilencesFromExtension(ctx)
	if err != nil {
		return err
	}
	id := strconv.FormatUint(uint64(silence.ID), 10)
	for i := range items {
		if strings.TrimSpace(items[i].Id) == id {
			items[i].AlertType = alertType
			items[i].StartAt = utils.FormatTimestamp(silence.CreatedAt)
			items[i].EndAt = utils.FormatTimestamp(silence.ExpiresAt)
			items[i].CreatedBy = strings.TrimSpace(silence.CreatedBy)
			return s.saveAlertingSilencesToExtension(ctx, items)
		}
	}
	items = append(items, Silence{
		Id:        id,
		AlertType: strings.TrimSpace(alertType),
		Matchers:  map[string]interface{}{},
		StartAt:   utils.FormatTimestamp(silence.CreatedAt),
		EndAt:     utils.FormatTimestamp(silence.ExpiresAt),
		CreatedBy: strings.TrimSpace(silence.CreatedBy),
	})
	return s.saveAlertingSilencesToExtension(ctx, items)
}

func (s *Service) removeAlertingSilenceFromExtension(ctx context.Context, silenceID string) error {
	id := strings.TrimSpace(silenceID)
	if id == "" {
		return nil
	}
	items, ok, err := s.loadAlertingSilencesFromExtension(ctx)
	if err != nil {
		return err
	}
	if !ok || len(items) == 0 {
		return nil
	}
	filtered := make([]Silence, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Id) == id {
			continue
		}
		filtered = append(filtered, item)
	}
	return s.saveAlertingSilencesToExtension(ctx, filtered)
}
