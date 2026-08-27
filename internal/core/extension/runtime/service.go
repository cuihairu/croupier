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
				SpecJSON:    jsonValue(`{"title":"Overview","route":"/analytics/overview","icon":"dashboard","order":10,"required_permission":"analytics.read"}`),
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.realtime",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Realtime","route":"/analytics/realtime","icon":"pulse","order":20,"required_permission":"analytics.read"}`),
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.retention",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Retention","route":"/analytics/retention","icon":"retention","order":30,"required_permission":"analytics.read"}`),
				Status:      "active",
			},
			{
				BindingType: "page",
				BindingKey:  "analytics.payments",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Payments","route":"/analytics/payments","icon":"payments","order":40,"required_permission":"analytics.read"}`),
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "analytics.filters",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"operations":["get","update"],"permissions":{"get":"analytics.read","update":"analytics.operate"},"config_keys":["filters"]}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.filters.get",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"get"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.filters.update",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"update"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "analytics.ingest.batch",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"ingest"}`),
				Status:      "active",
			},
		}
	}
	if strings.EqualFold(extID, "official.alerting") {
		return []model.ExtensionRuntimeBinding{
			{
				BindingType: "page",
				BindingKey:  "alerts.overview",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Alerts","route":"/alerts","icon":"alert","order":10,"required_permission":"alerts.read"}`),
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "alerts.management",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"operations":["list","silence","unsilence"],"permissions":{"list":"alerts.read","silence":"alerts.operate","unsilence":"alerts.operate"},"config_keys":["silence_rules"]}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "alerts.list",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"list"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "alerts.silence",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"silence"}`),
				Status:      "active",
			},
		}
	}
	if strings.EqualFold(extID, "official.notification") {
		return []model.ExtensionRuntimeBinding{
			{
				BindingType: "page",
				BindingKey:  "notifications.overview",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Notifications","route":"/ops/notifications","icon":"bell","order":10,"required_permission":"notifications.read"}`),
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "notifications.management",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"operations":["get","update"],"permissions":{"get":"notifications.read","update":"notifications.operate"},"config_keys":["enabled","channels","rules"]}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "notifications.get",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"get"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "notifications.update",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"update"}`),
				Status:      "active",
			},
		}
	}
	if strings.EqualFold(extID, "official.approval") {
		return []model.ExtensionRuntimeBinding{
			{
				BindingType: "page",
				BindingKey:  "approvals.overview",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Approvals","route":"/approvals","icon":"approval","order":10,"required_permission":"approvals.read"}`),
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "approvals.management",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"operations":["list","get","approve","reject"],"permissions":{"list":"approvals.read","get":"approvals.read","approve":"approvals.operate","reject":"approvals.operate"},"config_keys":["workflow","delegation"]}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "approvals.approve",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"approve"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "approvals.reject",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"reject"}`),
				Status:      "active",
			},
		}
	}
	if strings.EqualFold(extID, "official.backup-advanced") {
		return []model.ExtensionRuntimeBinding{
			{
				BindingType: "page",
				BindingKey:  "backups.overview",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"title":"Backups","route":"/backups","icon":"backup","order":10,"required_permission":"backups.read"}`),
				Status:      "active",
			},
			{
				BindingType: "capability",
				BindingKey:  "backups.management",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"operations":["list","create","delete","download"],"permissions":{"list":"backups.read","create":"backups.operate","delete":"backups.operate","download":"backups.read"},"config_keys":["schedule","retention","storage"]}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "backups.create",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"create"}`),
				Status:      "active",
			},
			{
				BindingType: "function",
				BindingKey:  "backups.delete",
				TargetRef:   targetRef,
				SpecJSON:    jsonValue(`{"driver":"workflow-driver","operation":"delete"}`),
				Status:      "active",
			},
		}
	}
	return []model.ExtensionRuntimeBinding{
		{
			BindingType: "function",
			BindingKey:  extID + ".default",
			TargetRef:   targetRef,
			SpecJSON:    jsonValue(`{"strategy":"default"}`),
			Status:      "active",
		},
	}
}

func jsonValue(v string) model.JSON {
	return model.JSON([]byte(v))
}
