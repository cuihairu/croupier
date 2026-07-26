package page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestServiceSaveDraftRequiresPageEditPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Schema:   testPageSchema(),
		Bindings: testPageBindings(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权编辑页面")
}

func TestServiceSaveDraftUsesContextActorAndWritesAudit(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit")
	revision := 0

	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Schema:   testPageSchema(),
		Bindings: testPageBindings(),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, resp.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "page_tester", draft.UpdatedBy)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageDraftSave},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "page_tester", records[0].Actor.ID)
	assert.Equal(t, "page", records[0].Resource.Type)
	assert.Equal(t, "player.manage", records[0].Resource.ID)
	assert.Equal(t, "demo-game", records[0].Resource.GameID)
	assert.Equal(t, "development", records[0].Resource.Environment)
	assert.Equal(t, "player.manage", records[0].Details["page_key"])
	assert.Equal(t, "demo-game", records[0].Details["game_id"])
	assert.Equal(t, "development", records[0].Details["env"])
}

func TestServicePublishRequiresPagePublishPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := saveTestPageDraft(t, service, ctx)

	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}

func TestServicePublishWritesActorAndAudit(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})

	require.NoError(t, err)
	assert.True(t, resp.Published)
	assert.Equal(t, revision, resp.PublishedVersion)

	published, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, "page_tester", published.PublishedBy)

	records, total, err := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPagePublish},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, records, 1)
	assert.Equal(t, "page_tester", records[0].Actor.ID)
	assert.Equal(t, "player.manage", records[0].Resource.ID)
	assert.Equal(t, revision, records[0].Details["published_version"])
}

func newPageTestService(t *testing.T, permissions ...string) (*Service, context.Context, *audit.InMemoryAuditStore) {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.query": {
				Enabled:      true,
				Version:      "1.0.0",
				Resource:     "player",
				Operation:    "query",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`,
			},
		},
	})

	admin := model.Admin{Username: "page_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "page_tester_role", Description: "page tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		grantPermission(t, db, role.ID, permissionID)
	}

	auditStore := audit.NewInMemoryAuditStore()
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:                     db,
		AdminModel:             model.NewAdminModel(db),
		RoleModel:              model.NewRoleModel(db),
		PermissionModel:        model.NewPermissionModel(db),
		PageSpecModel:          model.NewPageSpecModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		PageVersionModel:       model.NewPageVersionModel(db),
		RegistryStore:          store,
		AuditService:           audit.NewAuditService(auditStore, nil),
		Cache:                  nullCache,
		CacheHelper:            cache.NewCacheHelper(nullCache),
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", admin.Username)
	return NewService(svcCtx), ctx, auditStore
}

func saveTestPageDraft(t *testing.T, service *Service, ctx context.Context) int {
	t.Helper()

	revision := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Schema:   testPageSchema(),
		Bindings: testPageBindings(),
	})
	require.NoError(t, err)
	return resp.DraftRevision
}

func testPageSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"x-component":"ConsolePage",
		"x-component-props":{"schemaVersion":"formily-page:1"},
		"properties":{
			"query":{"type":"object","x-component":"QueryForm","x-component-props":{"bindingId":"player.query"}}
		}
	}`)
}

func testPageBindings() []spec.PageFunctionBinding {
	return []spec.PageFunctionBinding{
		{
			ID:         "player.query",
			FunctionID: "player.query",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
}

func grantPermission(t *testing.T, db *gorm.DB, roleID uint, permissionID string) {
	t.Helper()

	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return
	}
	parts := strings.SplitN(permissionID, ":", 2)
	action := "*"
	if len(parts) == 2 {
		action = parts[1]
	}
	permission := model.Permission{
		ID:       permissionID,
		Name:     permissionID,
		Resource: parts[0],
		Action:   action,
		Category: "dashboard",
	}
	require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error)
}
