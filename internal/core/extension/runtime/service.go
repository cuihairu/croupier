package runtime

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"gorm.io/gorm"
)

type Service struct {
	installationRepo *extensiongorm.InstallationRepo
	bindingRepo      *extensiongorm.BindingRepo
	eventRepo        *extensiongorm.EventRepo
}

type ReconcileResult struct {
	InstallationID uint
	Status         string
	Applied        int
	Failed         int
	Message        string
}

func NewService(installationRepo *extensiongorm.InstallationRepo, bindingRepo *extensiongorm.BindingRepo, eventRepo *extensiongorm.EventRepo) *Service {
	return &Service{
		installationRepo: installationRepo,
		bindingRepo:      bindingRepo,
		eventRepo:        eventRepo,
	}
}

func (s *Service) Reconcile(ctx context.Context, installationID uint) (*ReconcileResult, error) {
	if s == nil || s.installationRepo == nil || s.bindingRepo == nil || s.eventRepo == nil {
		return nil, gorm.ErrInvalidDB
	}
	item, err := s.installationRepo.GetByID(ctx, installationID)
	if err != nil {
		return nil, err
	}
	if item.Status == "" {
		item.Status = "installed"
	}
	if err := s.installationRepo.Save(ctx, item); err != nil {
		return nil, err
	}
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "function",
			BindingKey:  item.ExtensionID + ".default",
			TargetRef:   "extension:" + item.ExtensionID,
			SpecJSON:    `{"strategy":"default"}`,
			Status:      "active",
		},
	}
	if err := s.bindingRepo.ReplaceForInstallation(ctx, installationID, bindings); err != nil {
		return nil, err
	}
	_ = s.eventRepo.Create(ctx, &model.ExtensionEvent{
		InstallationID: installationID,
		EventType:      "reconcile",
		Level:          "info",
		Message:        "runtime reconciled",
	})
	return &ReconcileResult{
		InstallationID: installationID,
		Status:         item.Status,
		Applied:        len(bindings),
		Failed:         0,
		Message:        "reconciled",
	}, nil
}
