package alert

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestSilencesListPrefersExtensionConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
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
		Config: map[string]any{
			"silences": []map[string]any{
				{
					"id":        "s-1",
					"alertType": "cpu",
					"matchers":  map[string]any{},
					"startAt":   "2026-03-15 00:00:00",
					"endAt":     "2026-03-15 01:00:00",
					"createdBy": "tester",
				},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})
	if err != nil {
		t.Fatalf("SilencesList() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Id != "s-1" || resp.Items[0].AlertType != "cpu" {
		t.Fatalf("expected extension silences, got %#v", resp.Items)
	}
}

func TestSilenceSyncsToExtensionConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "a-1",
		Type:    "cpu",
		Level:   "warning",
		Message: "high cpu",
		Source:  "agent-1",
		Status:  "firing",
	}); err != nil {
		t.Fatalf("seed alert failed: %v", err)
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
		AlertModel: alertModel,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if err := s.Silence(context.WithValue(context.Background(), "username", "alice"), &AlertSilenceRequest{
		ID:       "a-1",
		Duration: 30,
		Reason:   "manual",
	}); err != nil {
		t.Fatalf("Silence() error = %v", err)
	}

	current, err := installationSvc.Get(context.Background(), installed.ID)
	if err != nil {
		t.Fatalf("installationSvc.Get() error = %v", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal([]byte(current.ConfigJSON), &config); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	raw, ok := config["silences"]
	if !ok || raw == nil {
		t.Fatalf("expected silences written into extension config, got %#v", config)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal silences failed: %v", err)
	}
	items := []Silence{}
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal silences failed: %v", err)
	}
	if len(items) == 0 || items[0].AlertType != "cpu" {
		t.Fatalf("unexpected silences config: %+v", items)
	}
}

// List tests

func TestList_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		AlertModel: nil,
	})

	resp, err := s.List(context.Background(), &AlertsListRequest{})

	if err == nil {
		t.Fatal("expected error for nil AlertModel")
	}
	if resp != nil {
		t.Fatal("expected nil response for error")
	}
}

func TestList_NilRequest(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	resp, err := s.List(context.Background(), nil)

	if err != nil {
		t.Fatalf("List(nil) error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestList_WithAlertsInDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create test alerts
	for i := 1; i <= 3; i++ {
		if err := alertModel.Create(context.Background(), &model.Alert{
			AlertID: fmt.Sprintf("alert-%d", i),
			Type:    "test",
			Level:   "info",
			Message: fmt.Sprintf("test alert %d", i),
			Status:  "active",
		}); err != nil {
			t.Fatalf("create alert failed: %v", err)
		}
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	resp, err := s.List(context.Background(), &AlertsListRequest{
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Total != 3 {
		t.Fatalf("expected total=3, got %d", resp.Total)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
}

// Silence tests

func TestSilence_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		AlertModel: nil,
	})

	err := s.Silence(context.Background(), &AlertSilenceRequest{ID: "test"})

	if err == nil {
		t.Fatal("expected error for nil AlertModel")
	}
}

func TestSilence_NilRequest(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.Silence(context.Background(), nil)

	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSilence_EmptyAlertID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "",
		Duration: 60,
		Reason:   "test",
	})

	if err == nil {
		t.Fatal("expected error for empty alert ID")
	}
}

func TestSilence_WhitespaceAlertID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "   ",
		Duration: 60,
		Reason:   "test",
	})

	if err == nil {
		t.Fatal("expected error for whitespace alert ID")
	}
}

func TestSilence_NonExistentAlert(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "non-existent",
		Duration: 60,
		Reason:   "test",
	})

	if err == nil {
		t.Fatal("expected error for non-existent alert")
	}
}

func TestSilence_NegativeDuration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "test-alert",
		Type:    "test",
		Level:   "info",
		Message: "test",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	// Negative duration should default to 60
	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "test-alert",
		Duration: -10,
		Reason:   "test",
	})

	if err != nil {
		t.Fatalf("Silence() with negative duration error = %v", err)
	}

	// Verify silence was created with default 60 minutes
	silences, err := alertModel.ListSilences(context.Background(), model.ListSilencesOptions{})
	if err != nil {
		t.Fatalf("ListSilences() error = %v", err)
	}
	if len(silences) != 1 {
		t.Fatalf("expected 1 silence, got %d", len(silences))
	}
	if silences[0].DurationMinute != 60 {
		t.Fatalf("expected default duration 60, got %d", silences[0].DurationMinute)
	}
}

// SilencesList tests

func TestSilencesList_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		AlertModel: nil,
		Extensions: nil,
	})

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})

	if err == nil {
		t.Fatal("expected error for nil AlertModel")
	}
	if resp != nil {
		t.Fatal("expected nil response for error")
	}
}

func TestSilencesList_WithEmptyDatabase(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
		Extensions: nil,
	})

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})

	if err != nil {
		t.Fatalf("SilencesList() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(resp.Items))
	}
}

// SilenceDelete tests

func TestSilenceDelete_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		AlertModel: nil,
	})

	err := s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: "1"})

	if err == nil {
		t.Fatal("expected error for nil AlertModel")
	}
}

func TestSilenceDelete_NilRequest(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.SilenceDelete(context.Background(), nil)

	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSilenceDelete_InvalidIDFormat(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	err = s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: "invalid"})

	if err == nil {
		t.Fatal("expected error for invalid ID format")
	}
}

func TestSilenceDelete_NonExistentID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: model.NewAlertModel(db),
	})

	// Deleting non-existent silence should not error
	err = s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: "99999"})

	if err != nil {
		t.Fatalf("SilenceDelete() non-existent error = %v", err)
	}
}

// Helper function tests

func TestFindActiveAlertingInstallation_NilService(t *testing.T) {
	var s *Service
	item, ok, err := s.findActiveAlertingInstallation(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nil service")
	}
	if item != nil {
		t.Fatal("expected nil item for nil service")
	}
}

func TestFindActiveAlertingInstallation_NilSvcCtx(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	item, ok, err := s.findActiveAlertingInstallation(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nil Extensions")
	}
	if item != nil {
		t.Fatal("expected nil item for nil Extensions")
	}
}

func TestFindActiveAlertingInstallation_NilExtensions(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		Extensions: nil,
	})
	item, ok, err := s.findActiveAlertingInstallation(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nil Extensions")
	}
	if item != nil {
		t.Fatal("expected nil item for nil Extensions")
	}
}

func TestFindActiveAlertingInstallation_NilInstallationService(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	})
	item, ok, err := s.findActiveAlertingInstallation(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nil Installation service")
	}
	if item != nil {
		t.Fatal("expected nil item for nil Installation service")
	}
}

func TestLoadAlertingSilencesFromExtension_NoInstallation(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	})
	items, ok, err := s.loadAlertingSilencesFromExtension(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nil Installation")
	}
	if items != nil {
		t.Fatal("expected nil items for nil Installation")
	}
}

func TestAppendAlertingSilenceToExtension_NilSilence(t *testing.T) {
	s := NewService(&svc.ServiceContext{})

	err := s.appendAlertingSilenceToExtension(context.Background(), "test", nil)

	if err != nil {
		t.Fatalf("appendAlertingSilenceToExtension(nil) error = %v", err)
	}
}

func TestRemoveAlertingSilenceFromExtension_EmptyID(t *testing.T) {
	s := NewService(&svc.ServiceContext{})

	err := s.removeAlertingSilenceFromExtension(context.Background(), "")

	if err != nil {
		t.Fatalf("removeAlertingSilenceFromExtension(empty) error = %v", err)
	}
}

func TestRemoveAlertingSilenceFromExtension_WhitespaceID(t *testing.T) {
	s := NewService(&svc.ServiceContext{})

	err := s.removeAlertingSilenceFromExtension(context.Background(), "   ")

	if err != nil {
		t.Fatalf("removeAlertingSilenceFromExtension(whitespace) error = %v", err)
	}
}

func TestRecordAlertingEvent_NoInstallation(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: nil,
		},
	})

	err := s.recordAlertingEvent(context.Background(), "test", "test event", "{}")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Additional tests for SilencesList with database

func TestSilencesList_WithDatabaseSilences(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create test alert
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "db-alert-1",
		Type:    "database",
		Level:   "info",
		Message: "test alert",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	// Get the alert for its ID
	alert, err := alertModel.FindByAlertID(context.Background(), "db-alert-1")
	if err != nil {
		t.Fatalf("find alert failed: %v", err)
	}

	// Create silences
	for i := 1; i <= 3; i++ {
		silence := &model.AlertSilence{
			AlertID:        alert.ID,
			Reason:         fmt.Sprintf("test silence %d", i),
			DurationMinute: 60,
			CreatedBy:      "tester",
		}
		if err := alertModel.CreateSilence(context.Background(), silence); err != nil {
			t.Fatalf("create silence failed: %v", err)
		}
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
		Extensions: nil, // No extension - should use database
	})

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})

	if err != nil {
		t.Fatalf("SilencesList() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
}

func TestSilencesList_WithDuplicateAlertIDs(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create test alert
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "dup-alert-1",
		Type:    "duplicate",
		Level:   "warning",
		Message: "test alert",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	alert, err := alertModel.FindByAlertID(context.Background(), "dup-alert-1")
	if err != nil {
		t.Fatalf("find alert failed: %v", err)
	}

	// Create multiple silences for the same alert
	for i := 1; i <= 3; i++ {
		silence := &model.AlertSilence{
			AlertID:        alert.ID,
			Reason:         fmt.Sprintf("dup silence %d", i),
			DurationMinute: 60,
			CreatedBy:      "tester",
		}
		if err := alertModel.CreateSilence(context.Background(), silence); err != nil {
			t.Fatalf("create silence failed: %v", err)
		}
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
		Extensions: nil,
	})

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})

	if err != nil {
		t.Fatalf("SilencesList() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// All 3 silences should be returned
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
}

func TestSilencesList_WithZeroAlertID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create silence with zero AlertID (orphaned)
	silence := &model.AlertSilence{
		AlertID:        0, // Zero alert ID
		Reason:         "orphaned silence",
		DurationMinute: 60,
		CreatedBy:      "tester",
	}
	if err := alertModel.CreateSilence(context.Background(), silence); err != nil {
		t.Fatalf("create silence failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
		Extensions: nil,
	})

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})

	if err != nil {
		t.Fatalf("SilencesList() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Orphaned silence should still be included, with empty AlertType
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item (orphaned silence), got %d", len(resp.Items))
	}
	if resp.Items[0].AlertType != "" {
		t.Fatalf("expected empty AlertType for orphaned silence, got '%s'", resp.Items[0].AlertType)
	}
}

func TestSilence_WithZeroDuration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "zero-duration-alert",
		Type:    "test",
		Level:   "info",
		Message: "test",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	// Zero duration should default to 60
	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "zero-duration-alert",
		Duration: 0,
		Reason:   "test",
	})

	if err != nil {
		t.Fatalf("Silence() with zero duration error = %v", err)
	}

	silences, err := alertModel.ListSilences(context.Background(), model.ListSilencesOptions{})
	if err != nil {
		t.Fatalf("ListSilences() error = %v", err)
	}
	if len(silences) != 1 || silences[0].DurationMinute != 60 {
		t.Fatalf("expected default duration 60, got %d", silences[0].DurationMinute)
	}
}

func TestSilence_WithWhitespaceReason(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "whitespace-reason-alert",
		Type:    "test",
		Level:   "info",
		Message: "test",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "whitespace-reason-alert",
		Duration: 60,
		Reason:   "  test reason  ",
	})

	if err != nil {
		t.Fatalf("Silence() error = %v", err)
	}

	silences, err := alertModel.ListSilences(context.Background(), model.ListSilencesOptions{})
	if err != nil {
		t.Fatalf("ListSilences() error = %v", err)
	}
	if len(silences) != 1 {
		t.Fatalf("expected 1 silence, got %d", len(silences))
	}
	// Reason should be trimmed
	if silences[0].Reason != "test reason" {
		t.Fatalf("expected trimmed reason 'test reason', got '%s'", silences[0].Reason)
	}
}

func TestSilence_WithWhitespaceAlertID(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "whitespace-id-alert",
		Type:    "test",
		Level:   "info",
		Message: "test",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	// Whitespace in ID should be trimmed
	err = s.Silence(context.Background(), &AlertSilenceRequest{
		ID:       "  whitespace-id-alert  ",
		Duration: 60,
		Reason:   "test",
	})

	if err != nil {
		t.Fatalf("Silence() error = %v", err)
	}
}

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)

	if s == nil {
		t.Fatal("expected non-nil service")
	}
	if s.svcCtx != svcCtx {
		t.Fatal("expected svcCtx to be set")
	}
}

func TestList_WithWhitespaceFilters(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create test alerts
	for i := 1; i <= 3; i++ {
		if err := alertModel.Create(context.Background(), &model.Alert{
			AlertID: fmt.Sprintf("filter-alert-%d", i),
			Type:    "test",
			Level:   "info",
			Message: "test",
			Status:  "active",
		}); err != nil {
			t.Fatalf("create alert failed: %v", err)
		}
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	// Test with whitespace in filters - should be trimmed
	resp, err := s.List(context.Background(), &AlertsListRequest{
		Level:   "  info  ",
		Status:  "  active  ",
		Page:    1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
}

func TestSilenceDelete_SuccessWithValidSilence(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	alertModel := model.NewAlertModel(db)
	// Create test alert
	if err := alertModel.Create(context.Background(), &model.Alert{
		AlertID: "delete-silence-alert",
		Type:    "test",
		Level:   "info",
		Message: "test",
		Status:  "active",
	}); err != nil {
		t.Fatalf("create alert failed: %v", err)
	}

	alert, err := alertModel.FindByAlertID(context.Background(), "delete-silence-alert")
	if err != nil {
		t.Fatalf("find alert failed: %v", err)
	}

	silence := &model.AlertSilence{
		AlertID:        alert.ID,
		Reason:         "to be deleted",
		DurationMinute: 60,
		CreatedBy:      "tester",
	}
	if err := alertModel.CreateSilence(context.Background(), silence); err != nil {
		t.Fatalf("create silence failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		AlertModel: alertModel,
	})

	silenceID := fmt.Sprintf("%d", silence.ID)
	err = s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: silenceID})

	if err != nil {
		t.Fatalf("SilenceDelete() error = %v", err)
	}

	// Verify silence was deleted
	silences, err := alertModel.ListSilences(context.Background(), model.ListSilencesOptions{})
	if err != nil {
		t.Fatalf("ListSilences() error = %v", err)
	}
	if len(silences) != 0 {
		t.Fatalf("expected 0 silences after delete, got %d", len(silences))
	}
}
