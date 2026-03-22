package ops

import (
	"context"
	"encoding/json"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRecordNotificationEvent(t *testing.T) {
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
		ExtensionID:    officialNotificationID,
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

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}
	if err := recordNotificationEvent(
		context.WithValue(context.Background(), "username", "alice"),
		svcCtx,
		"notifications_update",
		"notifications updated",
		`{"enabled":true,"channels":1,"rules":1}`,
	); err != nil {
		t.Fatalf("recordNotificationEvent() error = %v", err)
	}

	events, total, err := installationSvc.ListEvents(context.Background(), installed.ID, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if total < 2 || len(events) < 2 {
		t.Fatalf("expected install + notification events, total=%d len=%d", total, len(events))
	}
	found := false
	for _, event := range events {
		if event.EventType == "notifications_update" && event.CreatedBy == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected notifications_update event created by alice, events=%+v", events)
	}
}

func TestOpsNotificationsUpdateRecordsExtensionEvent(t *testing.T) {
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
		ExtensionID:    officialNotificationID,
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

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}
	_, err = opsNotificationsUpdate(context.WithValue(context.Background(), "username", "bob"), svcCtx, &OpsNotificationsUpdateRequest{
		Enabled: true,
		Channels: []OpsNotificationChannel{
			{ID: "ch-1", Type: "webhook", URL: "https://example.com/hook"},
		},
		Rules: []OpsNotificationRule{
			{Event: "alert.fired", Channels: []string{"ch-1"}},
		},
	})
	if err != nil {
		t.Fatalf("opsNotificationsUpdate() error = %v", err)
	}

	events, _, err := installationSvc.ListEvents(context.Background(), installed.ID, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "notifications_update" && event.CreatedBy == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected notifications_update event created by bob, events=%+v", events)
	}
}

func TestOpsNotificationsGetPrefersExtensionInstallationConfig(t *testing.T) {
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
		ExtensionID:    officialNotificationID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"enabled": true,
			"channels": []map[string]any{
				{"id": "ch-1", "type": "webhook", "url": "https://example.com/hook"},
			},
			"rules": []map[string]any{
				{"event": "alert.fired", "channels": []string{"ch-1"}},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}
	resp, err := opsNotificationsGet(context.Background(), svcCtx, &OpsNotificationsGetRequest{})
	if err != nil {
		t.Fatalf("opsNotificationsGet() error = %v", err)
	}

	if !resp.Enabled {
		t.Fatalf("expected enabled=true, got %#v", resp.Enabled)
	}
	channels := resp.Channels
	if len(channels) != 1 || channels[0].ID != "ch-1" {
		t.Fatalf("unexpected channels: %#v", channels)
	}
}

func TestOpsNotificationsUpdateWritesExtensionInstallationConfig(t *testing.T) {
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
		ExtensionID:    officialNotificationID,
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

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	}
	_, err = opsNotificationsUpdate(context.Background(), svcCtx, &OpsNotificationsUpdateRequest{
		Enabled: true,
		Channels: []OpsNotificationChannel{
			{ID: "ch-1", Type: "webhook", URL: "https://example.com/hook"},
		},
		Rules: []OpsNotificationRule{
			{Event: "alert.fired", Channels: []string{"ch-1"}},
		},
	})
	if err != nil {
		t.Fatalf("opsNotificationsUpdate() error = %v", err)
	}

	current, err := installationSvc.Get(context.Background(), installed.ID)
	if err != nil {
		t.Fatalf("installationSvc.Get() error = %v", err)
	}
	cfg := map[string]any{}
	if err := json.Unmarshal([]byte(current.ConfigJSON), &cfg); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	if enabled, _ := cfg["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled=true in config, got %#v", cfg["enabled"])
	}
	if channels, ok := cfg["channels"].([]interface{}); !ok || len(channels) != 1 {
		t.Fatalf("expected one channel in config, got %#v", cfg["channels"])
	}
}
