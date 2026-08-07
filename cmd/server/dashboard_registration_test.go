package main

import (
	"context"
	"strings"
	"testing"

	consoleapi "github.com/cuihairu/croupier/internal/api/console"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWireDashboardRegistrationPipelineGeneratesProposal(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	store := reg.NewStoreWithDB(db)
	svcCtx := &svc.ServiceContext{
		Config:        config.Config{},
		DB:            db,
		RegistryStore: store,
	}
	wireDashboardRegistrationPipeline(svcCtx)

	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Summary:      "Ban player",
				Operation:    "ban",
				Capability:   "action",
				Execution:    "sync",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
			},
		},
	})

	ctx := context.Background()
	_, err = model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.ban")
	require.NoError(t, err)
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	require.Equal(t, "operation--player.ban", proposal.PageKey)
}

func TestDashboardRegistrationProposalPublishesToConsoleMenu(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))

	store := reg.NewStoreWithDB(db)
	seedDashboardRegistrationUser(t, db, "dashboard_tester", "console:read", "pages:read")
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		Config:                 config.Config{},
		DB:                     db,
		AdminModel:             model.NewAdminModel(db),
		RoleModel:              model.NewRoleModel(db),
		PermissionModel:        model.NewPermissionModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		RegistryStore:          store,
		Cache:                  nullCache,
		CacheHelper:            cache.NewCacheHelper(nullCache),
	}
	wireDashboardRegistrationPipeline(svcCtx)

	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Summary:      "Ban player",
				Operation:    "ban",
				Capability:   "action",
				Execution:    "sync",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
			},
		},
	})

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "dashboard_tester")
	result, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)
	require.Equal(t, "operation--player.ban", result.PageKey)

	menu, err := consoleapi.NewService(svcCtx).Menu(ctx, &consoleapi.ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	require.Equal(t, "player", menu.Items[0].Key)
	require.Len(t, menu.Items[0].Children, 1)
	require.Equal(t, "operation--player.ban", menu.Items[0].Children[0].Key)
}

func seedDashboardRegistrationUser(t *testing.T, db *gorm.DB, username string, permissions ...string) {
	t.Helper()
	admin := model.Admin{Username: username, Status: 1, PasswordHash: "test"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: username + "_role", Description: "dashboard registration tester"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, permissionID := range permissions {
		permissionID = strings.TrimSpace(permissionID)
		if permissionID == "" {
			continue
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
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permissionID}).Error)
	}
}
