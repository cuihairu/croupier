package alert

import (
	"context"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFindActiveAlertingInstallation(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	_, err = installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAlertingID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	item, ok, err := s.findActiveAlertingInstallation(context.Background())
	if err != nil {
		t.Fatalf("findActiveAlertingInstallation() error = %v", err)
	}
	if !ok || item == nil || item.ExtensionID != officialAlertingID {
		t.Fatalf("expected active official.alerting installation, got ok=%v item=%#v", ok, item)
	}
}

func TestRecordAlertingEvent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAlertingID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}
	if installed == nil {
		t.Fatal("expected non-nil installation")
	}

	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if err := s.recordAlertingEvent(context.WithValue(context.Background(), "username", "alice"), "alerts_silence", "silenced", `{"id":"a1"}`); err != nil {
		t.Fatalf("recordAlertingEvent() error = %v", err)
	}

	events, total, err := installationSvc.ListEvents(context.Background(), installed.ID, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if total < 2 || len(events) < 2 {
		t.Fatalf("expected install + alerting events, total=%d len=%d", total, len(events))
	}
	found := false
	for _, event := range events {
		if event.EventType == "alerts_silence" && event.CreatedBy == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected alerts_silence event created by alice, events=%+v", events)
	}
}
