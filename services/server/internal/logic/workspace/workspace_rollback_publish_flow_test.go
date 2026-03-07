package workspace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWorkspaceRollbackThenPublishFlow(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkspaceConfig{}, &model.ConfigVersion{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	v1Payload := map[string]interface{}{
		"objectKey": "player",
		"title":     "玩家工作台 v1",
		"layout": map[string]interface{}{
			"type": "tabs",
			"tabs": []map[string]interface{}{},
		},
		"published": false,
		"status":    "draft",
	}
	v1JSON, _ := json.Marshal(v1Payload)

	v2Payload := map[string]interface{}{
		"objectKey": "player",
		"title":     "玩家工作台 v2",
		"layout": map[string]interface{}{
			"type": "tabs",
			"tabs": []map[string]interface{}{},
		},
		"published": false,
		"status":    "draft",
	}
	v2JSON, _ := json.Marshal(v2Payload)

	if err := db.Create(&model.ConfigVersion{
		Key:       workspaceVersionKey("player"),
		Version:   1,
		Value:     string(v1JSON),
		CreatedBy: "tester",
		Message:   "save v1",
	}).Error; err != nil {
		t.Fatalf("insert v1: %v", err)
	}
	if err := db.Create(&model.ConfigVersion{
		Key:       workspaceVersionKey("player"),
		Version:   2,
		Value:     string(v2JSON),
		CreatedBy: "tester",
		Message:   "save v2",
	}).Error; err != nil {
		t.Fatalf("insert v2: %v", err)
	}

	opsStore := svc.NewOpsStateStore(filepath.Join(t.TempDir(), "ops"))
	svcCtx := &svc.ServiceContext{
		WorkspaceConfigModel: model.NewWorkspaceConfigModel(db),
		ConfigVersionModel:   model.NewConfigVersionModel(db),
		OpsStateStore:        opsStore,
	}

	ctx := context.WithValue(context.Background(), "username", "qa_user")
	rollbackLogic := NewWorkspaceRollbackLogic(ctx, svcCtx)
	if _, err := rollbackLogic.Rollback("player", "1"); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	publishLogic := NewWorkspacePublishLogic(ctx, svcCtx)
	resp, err := publishLogic.WorkspacePublish(&types.WorkspacePublishRequest{
		ObjectKey:   "player",
		PublishedBy: "qa_user",
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if resp == nil || !resp.Published {
		t.Fatalf("publish response invalid: %+v", resp)
	}

	current, err := svcCtx.WorkspaceConfigModel.FindByObjectKey(context.Background(), "player")
	if err != nil {
		t.Fatalf("query current workspace config failed: %v", err)
	}
	if !current.Published {
		t.Fatalf("expected current workspace to be published")
	}

	auditState := opsStore.Snapshot().Audit.Entries
	if !containsWorkspaceAuditAction(auditState, "workspace.rollback") {
		t.Fatalf("workspace.rollback audit entry not found")
	}
	if !containsWorkspaceAuditAction(auditState, "workspace.publish") {
		t.Fatalf("workspace.publish audit entry not found")
	}
}

func containsWorkspaceAuditAction(entries []svc.OpsAuditEntry, action string) bool {
	for _, entry := range entries {
		if entry.Action == action {
			return true
		}
	}
	return false
}
