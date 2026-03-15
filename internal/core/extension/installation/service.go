package installation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"gorm.io/gorm"
)

type Service struct {
	installationRepo *extensiongorm.InstallationRepo
	eventRepo        *extensiongorm.EventRepo
	bindingRepo      *extensiongorm.BindingRepo
}

type InstallRequest struct {
	ExtensionID    string
	ReleaseVersion string
	ScopeType      string
	ScopeID        string
	TargetType     string
	TargetID       string
	Config         map[string]any
	SecretRefs     map[string]string
	Operator       string
}

type ListQuery struct {
	ExtensionID string
	ScopeType   string
	ScopeID     string
	TargetType  string
	TargetID    string
	Status      string
	Enabled     *bool
	Limit       int
	Offset      int
}

type EventListQuery struct {
	Level   string
	Keyword string
	Limit   int
	Offset  int
}

func NewService(installationRepo *extensiongorm.InstallationRepo, eventRepo *extensiongorm.EventRepo, bindingRepo *extensiongorm.BindingRepo) *Service {
	return &Service{
		installationRepo: installationRepo,
		eventRepo:        eventRepo,
		bindingRepo:      bindingRepo,
	}
}

func (s *Service) Install(ctx context.Context, req InstallRequest) (*model.ExtensionInstallation, error) {
	if s == nil || s.installationRepo == nil {
		return nil, gorm.ErrInvalidDB
	}
	cfgJSON, err := marshalJSON(req.Config)
	if err != nil {
		return nil, err
	}
	secretJSON, err := marshalJSON(req.SecretRefs)
	if err != nil {
		return nil, err
	}
	item := &model.ExtensionInstallation{
		InstallationKey: buildInstallationKey(req),
		ExtensionID:     req.ExtensionID,
		ReleaseVersion:  req.ReleaseVersion,
		ScopeType:       req.ScopeType,
		ScopeID:         req.ScopeID,
		TargetType:      req.TargetType,
		TargetID:        req.TargetID,
		Status:          "installed",
		DesiredState:    "disabled",
		Enabled:         false,
		ConfigJSON:      cfgJSON,
		SecretRefsJSON:  secretJSON,
		InstalledBy:     req.Operator,
		InstalledAtUnix: time.Now().Unix(),
	}
	if err := s.installationRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	_ = s.appendEvent(ctx, item.ID, "install", "info", "extension installed", req.Operator, "")
	return item, nil
}

func (s *Service) List(ctx context.Context, q ListQuery) ([]model.ExtensionInstallation, int64, error) {
	if s == nil || s.installationRepo == nil {
		return []model.ExtensionInstallation{}, 0, nil
	}
	return s.installationRepo.List(ctx, extensiongorm.InstallationListQuery{
		ExtensionID: q.ExtensionID,
		ScopeType:   q.ScopeType,
		ScopeID:     q.ScopeID,
		TargetType:  q.TargetType,
		TargetID:    q.TargetID,
		Status:      q.Status,
		Enabled:     q.Enabled,
		Limit:       q.Limit,
		Offset:      q.Offset,
	})
}

func (s *Service) Get(ctx context.Context, id uint) (*model.ExtensionInstallation, error) {
	if s == nil || s.installationRepo == nil {
		return nil, gorm.ErrInvalidDB
	}
	return s.installationRepo.GetByID(ctx, id)
}

func (s *Service) UpdateConfig(ctx context.Context, id uint, config map[string]any, secretRefs map[string]string, operator string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	cfgJSON, err := marshalJSON(config)
	if err != nil {
		return err
	}
	secretJSON, err := marshalJSON(secretRefs)
	if err != nil {
		return err
	}
	item.ConfigJSON = cfgJSON
	item.SecretRefsJSON = secretJSON
	if err := s.installationRepo.Save(ctx, item); err != nil {
		return err
	}
	return s.appendEvent(ctx, item.ID, "update_config", "info", "extension config updated", operator, "")
}

func (s *Service) Enable(ctx context.Context, id uint, operator string) error {
	return s.updateState(ctx, id, true, "enabled", "enabled", "enable", operator)
}

func (s *Service) Disable(ctx context.Context, id uint, operator string) error {
	return s.updateState(ctx, id, false, "disabled", "disabled", "disable", operator)
}

func (s *Service) Upgrade(ctx context.Context, id uint, version, operator string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	item.ReleaseVersion = version
	item.Status = "enabled"
	if err := s.installationRepo.Save(ctx, item); err != nil {
		return err
	}
	return s.appendEvent(ctx, item.ID, "upgrade", "info", "extension upgraded", operator, version)
}

func (s *Service) Uninstall(ctx context.Context, id uint, operator string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	item.Status = "uninstalled"
	item.DesiredState = "uninstalled"
	item.Enabled = false
	if err := s.installationRepo.Save(ctx, item); err != nil {
		return err
	}
	if s.bindingRepo != nil {
		if err := s.bindingRepo.ReplaceForInstallation(ctx, id, []model.ExtensionRuntimeBinding{}); err != nil {
			return err
		}
	}
	return s.appendEvent(ctx, item.ID, "uninstall", "info", "extension uninstalled", operator, "")
}

func (s *Service) ListEvents(ctx context.Context, id uint, q EventListQuery) ([]model.ExtensionEvent, int64, error) {
	if s == nil || s.eventRepo == nil {
		return []model.ExtensionEvent{}, 0, nil
	}
	events, total, err := s.eventRepo.List(ctx, extensiongorm.EventListQuery{
		InstallationID: id,
		Level:          q.Level,
		Keyword:        q.Keyword,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (s *Service) ListBindings(ctx context.Context, id uint) ([]model.ExtensionRuntimeBinding, error) {
	if s == nil || s.bindingRepo == nil {
		return []model.ExtensionRuntimeBinding{}, nil
	}
	return s.bindingRepo.ListByInstallationID(ctx, id)
}

func (s *Service) RecordEvent(ctx context.Context, installationID uint, eventType, level, message, createdBy, payload string) error {
	return s.appendEvent(ctx, installationID, eventType, level, message, createdBy, payload)
}

func (s *Service) updateState(ctx context.Context, id uint, enabled bool, status, desiredState, eventType, operator string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	item.Enabled = enabled
	item.Status = status
	item.DesiredState = desiredState
	if err := s.installationRepo.Save(ctx, item); err != nil {
		return err
	}
	return s.appendEvent(ctx, item.ID, eventType, "info", fmt.Sprintf("extension %s", status), operator, "")
}

func (s *Service) appendEvent(ctx context.Context, installationID uint, eventType, level, message, createdBy, payload string) error {
	if s == nil || s.eventRepo == nil {
		return nil
	}
	return s.eventRepo.Create(ctx, &model.ExtensionEvent{
		InstallationID: installationID,
		EventType:      eventType,
		Level:          level,
		Message:        message,
		CreatedBy:      createdBy,
		PayloadJSON:    payload,
	})
}

func buildInstallationKey(req InstallRequest) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", req.ExtensionID, req.ScopeType, req.ScopeID, req.TargetType, req.TargetID, req.ReleaseVersion)
}

func marshalJSON(v any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
