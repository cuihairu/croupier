package runtime

import (
	"context"
	"strings"

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
	bindings := buildRuntimeBindings(item)
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

func buildRuntimeBindings(item *model.ExtensionInstallation) []model.ExtensionRuntimeBinding {
	if item == nil {
		return []model.ExtensionRuntimeBinding{}
	}
	extID := strings.TrimSpace(item.ExtensionID)
	targetRef := "extension:" + extID
	if strings.EqualFold(extID, "official.analytics") {
		return []model.ExtensionRuntimeBinding{
			{
				BindingType: "page",
				BindingKey:  "analytics.overview",
				TargetRef:   targetRef,
				SpecJSON:    `{"title":"Overview","route":"/analytics/overview","icon":"dashboard","order":10}`,
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.realtime",
				TargetRef:   targetRef,
				SpecJSON:    `{"title":"Realtime","route":"/analytics/realtime","icon":"pulse","order":20}`,
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.retention",
				TargetRef:   targetRef,
				SpecJSON:    `{"title":"Retention","route":"/analytics/retention","icon":"retention","order":30}`,
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.payments",
				TargetRef:   targetRef,
				SpecJSON:    `{"title":"Payments","route":"/analytics/payments","icon":"payments","order":40}`,
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "analytics.filters",
				TargetRef:   targetRef,
				SpecJSON:    `{"operations":["get","update"]}`,
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.filters.get",
				TargetRef:   targetRef,
				SpecJSON:    `{"driver":"workflow-driver","operation":"get"}`,
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.filters.update",
				TargetRef:   targetRef,
				SpecJSON:    `{"driver":"workflow-driver","operation":"update"}`,
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.ingest.batch",
				TargetRef:   targetRef,
				SpecJSON:    `{"driver":"workflow-driver","operation":"ingest"}`,
				Status:      "active",
			},
		}
	}
	return []model.ExtensionRuntimeBinding{
		{
			BindingType: "function",
			BindingKey:  extID + ".default",
			TargetRef:   targetRef,
			SpecJSON:    `{"strategy":"default"}`,
			Status:      "active",
		},
	}
}
