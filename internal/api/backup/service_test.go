package backup

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

func TestCreateRecordsExtensionEvent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Backup{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialBackupAdvancedID,
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
		BackupModel: model.NewBackupModel(db),
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.Create(context.WithValue(context.Background(), "username", "alice"), &BackupCreateRequest{
		Name: "nightly",
		Type: "full",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp == nil || resp.Backup.Id == "" {
		t.Fatalf("expected created backup id, got %+v", resp)
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
		if event.EventType == "backups_create" && event.CreatedBy == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected backups_create event created by alice, events=%+v", events)
	}
}

func TestDeleteRecordsExtensionEvent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Backup{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialBackupAdvancedID,
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

	backupModel := model.NewBackupModel(db)
	seed := &model.Backup{
		BackupID: "bk-1",
		Name:     "seed",
		Type:     "full",
		Status:   "done",
	}
	if err := backupModel.Create(context.Background(), seed); err != nil {
		t.Fatalf("seed backup failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		BackupModel: backupModel,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if err := s.Delete(context.WithValue(context.Background(), "username", "bob"), &BackupDeleteRequest{ID: "bk-1"}); err != nil {
		t.Fatalf("Delete() error = %v", err)
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
		if event.EventType == "backups_delete" && event.CreatedBy == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected backups_delete event created by bob, events=%+v", events)
	}
}

func TestListPrefersExtensionInstallationConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Backup{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	_, err = installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialBackupAdvancedID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"backups": []map[string]any{
				{"id": "bk-ext-1", "name": "ext-backup", "type": "full", "status": "done", "createdAt": "2026-03-15 00:00:00"},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		BackupModel: model.NewBackupModel(db),
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.List(context.Background(), &BackupsListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Id != "bk-ext-1" {
		t.Fatalf("expected extension backups list, got %#v", resp.Items)
	}
}

func TestCreateSyncsBackupToExtensionConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Backup{}, &model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialBackupAdvancedID,
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
		BackupModel: model.NewBackupModel(db),
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	created, err := s.Create(context.Background(), &BackupCreateRequest{Name: "sync-me", Type: "full"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created == nil || created.Backup.Id == "" {
		t.Fatalf("expected created backup id, got %+v", created)
	}

	current, err := installationSvc.Get(context.Background(), installed.ID)
	if err != nil {
		t.Fatalf("installationSvc.Get() error = %v", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal([]byte(current.ConfigJSON), &config); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	raw, ok := config["backups"]
	if !ok || raw == nil {
		t.Fatalf("expected backups written into extension config, got %#v", config)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal backups failed: %v", err)
	}
	items := []Backup{}
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal backups failed: %v", err)
	}
	if len(items) == 0 || items[0].Id != created.Backup.Id {
		t.Fatalf("unexpected backups config: %+v", items)
	}
}
