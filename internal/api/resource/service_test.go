package resource

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestServiceListCollectsRegistryDescriptorV2Metadata(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player"},
				Summary:      "List players",
				Description:  "List player accounts",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"}}}`,
				Resource:     "player",
				Risk:         "safe",
				Operation:    "list",
				Permission:   "player:list",
			},
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player", "moderation"},
				Summary:      "Ban player",
				Description:  "Ban a player account",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Resource:     "player",
				Risk:         "danger",
				Operation:    "ban",
				Permission:   "player:ban",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:read")
	service := NewService(svcCtx)
	resp, err := service.List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	player := resp.Items[0]
	assert.Equal(t, "player", player.Key)
	assert.Equal(t, "player", player.Labels["zh-CN"])
	assert.Equal(t, "player", player.Category.Key)
	assert.Equal(t, "player", player.Category.Labels["zh-CN"])
	require.Len(t, player.Operations, 2)

	ops := map[string]spec.OperationSpec{}
	for _, op := range player.Operations {
		ops[op.FunctionID] = op
	}

	listOp := ops["player.list"]
	assert.Equal(t, "list", listOp.Operation)
	assert.Equal(t, "player:list", listOp.Permission)
	assert.Empty(t, listOp.Diagnostics)

	banOp := ops["player.ban"]
	assert.Equal(t, "ban", banOp.Operation)
	assert.Equal(t, spec.RiskDanger, banOp.Risk)
	assert.Equal(t, "player:ban", banOp.Permission)
	assert.Empty(t, banOp.Diagnostics)
}

func TestServiceGeneratedPagesCreatesConservativeOperationCandidates(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "list",
			},
			"player.ban": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "ban",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "pages:edit")
	service := NewService(svcCtx)
	resp, err := service.GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "player.ban", page.PageKey)
	assert.Equal(t, "player", page.Category.Key)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	assert.Contains(t, string(page.Schema), `"x-component":"ResultPanel"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.main"`)
	assert.Equal(t, "needs_review", page.Quality)
	assert.NotContains(t, string(page.Schema), `"functionId"`)
	assert.NotContains(t, string(page.Schema), `"operation":"update"`)
}

func TestServiceGeneratedPagesDoesNotGuessTableContract(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "list",
			},
		},
	})

	svcCtx, ctx := newResourceTestServiceContext(t, store, "resources:diagnose")
	service := NewService(svcCtx)
	resp, err := service.GeneratedPages(ctx, &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "needs_review", page.Quality)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	require.NotEmpty(t, page.Diagnostics)
	assert.Equal(t, "page_contract_missing", page.Diagnostics[0].Code)
}

func TestServiceListRequiresResourcePermission(t *testing.T) {
	store := reg.NewStore()
	svcCtx, ctx := newResourceTestServiceContext(t, store)

	_, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看资源")
}

func newResourceTestServiceContext(t *testing.T, store *reg.Store, permissions ...string) (*svc.ServiceContext, context.Context) {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	admin := model.Admin{Username: "resource_tester", Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)

	role := model.Role{Name: "resource_tester_role", Description: "resource tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)

	for _, permissionID := range permissions {
		permissionID = strings.TrimSpace(permissionID)
		if permissionID == "" {
			continue
		}
		permission := model.Permission{
			ID:       permissionID,
			Name:     permissionID,
			Resource: strings.SplitN(permissionID, ":", 2)[0],
			Action:   "read",
			Category: "dashboard",
		}
		require.NoError(t, db.Where("id = ?", permission.ID).FirstOrCreate(&permission).Error)
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permissionID}).Error)
	}

	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		RegistryStore:   store,
		Cache:           nullCache,
		CacheHelper:     cache.NewCacheHelper(nullCache),
	}
	ctx := context.WithValue(context.Background(), "username", admin.Username)
	return svcCtx, ctx
}
