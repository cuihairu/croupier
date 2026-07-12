package ops

import (
	"context"
	"errors"
	"strconv"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// Alert operations sub-service

type AlertService struct {
	svcCtx *svc.ServiceContext
}

func NewAlertService(svcCtx *svc.ServiceContext) *AlertService {
	return &AlertService{svcCtx: svcCtx}
}

func (s *AlertService) List(ctx context.Context, gameId, env, status string) ([]OpsAlert, error) {
	if s.svcCtx.AlertModel == nil {
		return nil, errors.New("alert model unavailable")
	}

	alerts, _, err := s.svcCtx.AlertModel.List(ctx, model.ListAlertsOptions{
		PaginationOptions: model.NewPagination(1, 100),
		Status: status,
	})
	if err != nil {
		return nil, err
	}

	items := make([]OpsAlert, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, OpsAlert{
			Severity: a.Level,
			Service:  a.Type,
			Summary:  a.Message,
			Labels: map[string]interface{}{
				"id":     strconv.FormatUint(uint64(a.ID), 10),
				"type":   a.Type,
				"status": a.Status,
			},
			Annotations: map[string]interface{}{
				"createdAt": utils.FormatTimestamp(a.CreatedAt),
				"updatedAt": utils.FormatTimestamp(a.UpdatedAt),
			},
		})
	}

	return items, nil
}

func (s *AlertService) Silence(ctx context.Context, alertId string, duration int, comment string) (string, error) {
	if s.svcCtx.AlertModel == nil {
		return "", errors.New("alert model unavailable")
	}

	// Parse the alert ID
	alertIDUint, err := strconv.ParseUint(alertId, 10, 32)
	if err != nil {
		return "", errors.New("invalid alert ID")
	}

	silence := &model.AlertSilence{
		AlertID:        uint(alertIDUint),
		Reason:         comment,
		DurationMinute: duration,
	}

	if err := s.svcCtx.AlertModel.CreateSilence(ctx, silence); err != nil {
		return "", err
	}

	return strconv.FormatUint(uint64(silence.ID), 10), nil
}

func (s *AlertService) DeleteSilence(ctx context.Context, silenceId string) error {
	if s.svcCtx.AlertModel == nil {
		return errors.New("alert model unavailable")
	}

	id, err := strconv.ParseUint(silenceId, 10, 32)
	if err != nil {
		return errors.New("invalid silence ID")
	}

	return s.svcCtx.AlertModel.DeleteSilence(ctx, uint(id))
}

func (s *AlertService) ListSilences(ctx context.Context, gameId string) ([]OpsAlert, error) {
	if s.svcCtx.AlertModel == nil {
		return nil, errors.New("alert model unavailable")
	}

	silences, err := s.svcCtx.AlertModel.ListSilences(ctx, model.ListSilencesOptions{
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}

	items := make([]OpsAlert, 0, len(silences))
	for _, s := range silences {
		items = append(items, OpsAlert{
			Severity: "info",
			Service:  "silence",
			Summary:  s.Reason,
			Labels: map[string]interface{}{
				"id":     strconv.FormatUint(uint64(s.ID), 10),
				"status": "active",
			},
			Annotations: map[string]interface{}{
				"createdAt": utils.FormatTimestamp(s.CreatedAt),
			},
		})
	}

	return items, nil
}
