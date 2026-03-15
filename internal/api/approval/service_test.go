package approval

import (
	"context"
	"encoding/json"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestApproveRecordsExtensionEvent(t *testing.T) {
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
		ExtensionID:    officialApprovalID,
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

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-1", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Approve(context.WithValue(context.Background(), "username", "alice"), &ApprovalApproveRequest{ID: "ap-1"}); err != nil {
		t.Fatalf("Approve() error = %v", err)
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
		if event.EventType == "approvals_approve" && event.CreatedBy == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected approvals_approve event created by alice, events=%+v", events)
	}
}

func TestRejectRecordsExtensionEvent(t *testing.T) {
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
		ExtensionID:    officialApprovalID,
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

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-2", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Reject(context.WithValue(context.Background(), "username", "bob"), &ApprovalRejectRequest{ID: "ap-2", Reason: "invalid"}); err != nil {
		t.Fatalf("Reject() error = %v", err)
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
		if event.EventType == "approvals_reject" && event.CreatedBy == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected approvals_reject event created by bob, events=%+v", events)
	}
}

func TestListPrefersExtensionInstallationConfig(t *testing.T) {
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
		ExtensionID:    officialApprovalID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"approvals": []map[string]any{
				{
					"id":         "ap-ext-1",
					"actor":      "tester",
					"state":      "pending",
					"created_at": "2026-03-15 00:00:00",
					"updated_at": "2026-03-15 00:00:00",
				},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Approvals) != 1 || resp.Approvals[0].ID != "ap-ext-1" {
		t.Fatalf("expected extension approvals list, got %#v", resp.Approvals)
	}
}

func TestApproveSyncsApprovalToExtensionConfig(t *testing.T) {
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
		ExtensionID:    officialApprovalID,
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

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-sync-1", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Approve(context.Background(), &ApprovalApproveRequest{ID: "ap-sync-1"}); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	current, err := installationSvc.Get(context.Background(), installed.ID)
	if err != nil {
		t.Fatalf("installationSvc.Get() error = %v", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal([]byte(current.ConfigJSON), &config); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	raw, ok := config["approvals"]
	if !ok || raw == nil {
		t.Fatalf("expected approvals written into extension config, got %#v", config)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal approvals failed: %v", err)
	}
	items := []Approval{}
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal approvals failed: %v", err)
	}
	if len(items) == 0 || items[0].ID != "ap-sync-1" || items[0].State != "approved" {
		t.Fatalf("unexpected approvals config: %+v", items)
	}
}
