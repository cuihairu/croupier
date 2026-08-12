package game

import (
	"context"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGameTestDB(t *testing.T) (*gorm.DB, *svc.ServiceContext) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)
	gameModel := model.NewGameModel(db)

	admin := &model.Admin{
		Username: "testadmin",
		Nickname: "Test Admin",
		Status:   1,
	}
	err = adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = roleModel.Create(nil, role)
	require.NoError(t, err)

	err = adminModel.AssignRole(nil, admin.ID, role.ID)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(nil, role.ID, []string{
		"admin:all", "games:read", "games:manage",
	})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "games:read", Name: "Games Read", Resource: "games", Action: "read", Category: "game"},
		{ID: "games:manage", Name: "Games Manage", Resource: "games", Action: "manage", Category: "game"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	svcCtx := &svc.ServiceContext{
		DB:                db,
		AdminModel:        adminModel,
		RoleModel:         roleModel,
		PermissionModel:   permissionModel,
		GameModel:         gameModel,
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
	}

	return db, svcCtx
}

func createGameTestContext(t *testing.T, db *gorm.DB) context.Context {
	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)
	return ctx
}

func TestService_List_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	// Create test games
	gameModel := svcCtx.GameModel
	game1 := &model.Game{Name: "game1", AliasName: "Game 1", Status: "dev"}
	err := gameModel.Create(ctx, game1)
	require.NoError(t, err)

	game2 := &model.Game{Name: "game2", AliasName: "Game 2", Status: "running"}
	err = gameModel.Create(ctx, game2)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &GamesListRequest{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 2)
}

func TestService_List_WithStatusFilter_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game1 := &model.Game{Name: "devgame", AliasName: "Dev Game", Status: "dev"}
	err := gameModel.Create(ctx, game1)
	require.NoError(t, err)

	game2 := &model.Game{Name: "rungame", AliasName: "Run Game", Status: "running"}
	err = gameModel.Create(ctx, game2)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &GamesListRequest{Page: 1, PageSize: 10, Status: "dev"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 1)
}

func TestService_List_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)

	ctx := context.WithValue(context.Background(), "username", "nobody")

	service := NewService(svcCtx)

	_, err := service.List(ctx, &GamesListRequest{Page: 1, PageSize: 10})
	assert.Error(t, err)
}

func TestService_Create_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:      "newgame",
		AliasName: "New Game",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "newgame", resp.Game.Name)
	assert.Equal(t, "New Game", resp.Game.AliasName)
	assert.Equal(t, "dev", resp.Game.Status)
}

func TestService_Create_EmptyName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.Create(ctx, &GameCreateRequest{Name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "游戏名称不能为空")
}

func TestService_Create_InvalidName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "invalid name!"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "游戏名称仅支持")
}

func TestService_Create_DuplicateName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "dupgame", AliasName: "Dup", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.Create(ctx, &GameCreateRequest{Name: "dupgame"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestService_Create_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)

	ctx := context.WithValue(context.Background(), "username", "nobody")

	service := NewService(svcCtx)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "test"})
	assert.Error(t, err)
}

func TestService_Detail_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "detailgame", AliasName: "Detail", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &GameDetailRequest{ID: strconv.FormatUint(uint64(game.ID), 10)})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "detailgame", resp.Game.Name)
}

func TestService_Detail_NotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.Detail(ctx, &GameDetailRequest{ID: "99999"})
	assert.Error(t, err)
}

func TestService_Detail_InvalidID_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.Detail(ctx, &GameDetailRequest{ID: "invalid"})
	assert.Error(t, err)
}

func TestService_Update_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "upgame", AliasName: "Up", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.FormatUint(uint64(game.ID), 10),
		AliasName: "Updated Game",
		Status:    "running",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Updated Game", resp.Game.AliasName)
	assert.Equal(t, "running", resp.Game.Status)
}

func TestService_Update_NoFields_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "nofield", AliasName: "NoField", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.Update(ctx, &GameUpdateRequest{
		ID: strconv.FormatUint(uint64(game.ID), 10),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请提供需要更新的字段")
}

func TestService_Update_InvalidStatus_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "invstatus", AliasName: "InvStatus", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.Update(ctx, &GameUpdateRequest{
		ID:     strconv.FormatUint(uint64(game.ID), 10),
		Status: "invalid_status",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的游戏状态")
}

func TestService_Update_DuplicateName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game1 := &model.Game{Name: "gamea", AliasName: "A", Status: "dev"}
	err := gameModel.Create(ctx, game1)
	require.NoError(t, err)

	game2 := &model.Game{Name: "gameb", AliasName: "B", Status: "dev"}
	err = gameModel.Create(ctx, game2)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.Update(ctx, &GameUpdateRequest{
		ID:   strconv.FormatUint(uint64(game2.ID), 10),
		Name: "gamea",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestService_Delete_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "delgame", AliasName: "Del", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	err = service.Delete(ctx, &GameDeleteRequest{ID: strconv.FormatUint(uint64(game.ID), 10)})
	assert.NoError(t, err)
}

func TestService_Delete_InvalidID_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	err := service.Delete(ctx, &GameDeleteRequest{ID: "invalid"})
	assert.Error(t, err)
}

func TestService_EnvsList_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envgame", AliasName: "Env", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Production"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: strconv.FormatUint(uint64(game.ID), 10)})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
	assert.Equal(t, "prod", resp.Envs[0].Env)
}

func TestService_EnvsList_InvalidID_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: "invalid"})
	assert.Error(t, err)
}

func TestService_EnvAdd_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envaddgame", AliasName: "EnvAdd", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "staging",
		Type: "Staging Environment",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
	assert.Equal(t, "staging", resp.Envs[0].Env)
}

func TestService_EnvAdd_EmptyName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envaddempty", AliasName: "EnvAddEmpty", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "环境名称不能为空")
}

func TestService_EnvAdd_Duplicate_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envadddup", AliasName: "EnvAddDup", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Production"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "prod",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestService_EnvUpdate_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envupgame", AliasName: "EnvUp", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Production"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "production",
		Type:  "Updated Production",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
	assert.Equal(t, "production", resp.Envs[0].Env)
}

func TestService_EnvUpdate_NotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envupnotfound", AliasName: "EnvUpNotFound", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "nonexistent",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestService_EnvUpdate_DuplicateName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envupdup", AliasName: "EnvUpDup", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "dev", Description: "Development"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "dev",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestService_EnvDelete_Success_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envdelgame", AliasName: "EnvDel", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Production"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 0)
	binding, err := gameModel.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	assert.Nil(t, binding)
}

func TestService_EnvChangesKeepBindingsSynchronized_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)
	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envbindingsync", AliasName: "EnvBindingSync", Status: "dev"}
	require.NoError(t, gameModel.Create(ctx, game))

	service := NewService(svcCtx)
	_, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "prod",
		Type: "Production",
	})
	require.NoError(t, err)

	binding, err := gameModel.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	originalDBName := binding.DatabaseName
	assert.Equal(t, "Production", binding.Description)

	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Type:  "Primary production",
	})
	require.NoError(t, err)
	binding, err = gameModel.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, originalDBName, binding.DatabaseName)
	assert.Equal(t, "Primary production", binding.Description)

	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "stage",
	})
	require.NoError(t, err)
	oldBinding, err := gameModel.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	assert.Nil(t, oldBinding)
	binding, err = gameModel.FindEnvBinding(ctx, game.GameID, "stage")
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, service.deriveGameDBName(game.GameID, "stage"), binding.DatabaseName)

	updated, err := gameModel.FindOne(ctx, game.ID)
	require.NoError(t, err)
	envs, err := updated.GetEnvs()
	require.NoError(t, err)
	require.Len(t, envs, 1)
	assert.Equal(t, "stage", envs[0].Env)
}

func TestGameModel_BackfillEnvBindingsIsIdempotent_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)
	game := &model.Game{Name: "legacyenvgame", AliasName: "LegacyEnv", Status: "dev"}
	require.NoError(t, game.SetEnvs([]model.GameEnv{
		{Env: "prod", Description: "Production", Color: "#111111"},
		{Env: "stage", Description: "Staging", Color: "#222222"},
	}))
	require.NoError(t, svcCtx.GameModel.Create(ctx, game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(
		ctx, game.GameID, "prod", "custom_legacy_prod", "Existing", "#333333",
	))

	created, err := svcCtx.GameModel.BackfillEnvBindings(ctx, func(gameID, env string) string {
		return "game_" + gameID + "_" + env
	})
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	prod, err := svcCtx.GameModel.FindEnvBinding(ctx, game.GameID, "prod")
	require.NoError(t, err)
	require.NotNil(t, prod)
	assert.Equal(t, "custom_legacy_prod", prod.DatabaseName)
	assert.Equal(t, "Existing", prod.Description)
	stage, err := svcCtx.GameModel.FindEnvBinding(ctx, game.GameID, "stage")
	require.NoError(t, err)
	require.NotNil(t, stage)
	assert.Equal(t, "game_legacyenvgame_stage", stage.DatabaseName)
	assert.Equal(t, "Staging", stage.Description)

	created, err = svcCtx.GameModel.BackfillEnvBindings(ctx, func(gameID, env string) string {
		return "changed_" + gameID + "_" + env
	})
	require.NoError(t, err)
	assert.Equal(t, 0, created)
	stage, err = svcCtx.GameModel.FindEnvBinding(ctx, game.GameID, "stage")
	require.NoError(t, err)
	assert.Equal(t, "game_legacyenvgame_stage", stage.DatabaseName)
}

func TestService_EnvDelete_NotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envdelnotfound", AliasName: "EnvDelNotFound", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "nonexistent",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestHelpers_SanitizeGameName_Extra(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "mygame", false},
		{"with numbers", "game123", false},
		{"with special chars", "my-game_2@v1", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"with spaces", "my game", true},
		{"with special", "my!game", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeGameName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, result)
			}
		})
	}
}

func TestHelpers_SanitizeStatus_Extra(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty", "", "", false},
		{"dev", "dev", "dev", false},
		{"running", "running", "running", false},
		{"online", "online", "online", false},
		{"offline", "offline", "offline", false},
		{"maintenance", "maintenance", "maintenance", false},
		{"invalid", "invalid", "", true},
		{"whitespace", "  ", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeStatus(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestHelpers_FindEnvIndex_Extra(t *testing.T) {
	envs := []model.GameEnv{
		{Env: "prod"},
		{Env: "dev"},
		{Env: "staging"},
	}

	assert.Equal(t, 0, findEnvIndex(envs, "prod"))
	assert.Equal(t, 1, findEnvIndex(envs, "dev"))
	assert.Equal(t, 2, findEnvIndex(envs, "staging"))
	assert.Equal(t, -1, findEnvIndex(envs, "nonexistent"))
	assert.Equal(t, 0, findEnvIndex(envs, "PROD")) // case insensitive
}

func TestHelpers_EnsureEnvName_Extra(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "prod", "prod", false},
		{"with spaces", "  prod  ", "prod", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ensureEnvName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestHelpers_ConvertGameEnvs_Extra(t *testing.T) {
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production", Color: "#ff0000"},
		{Env: "dev", Description: "Development"},
	}

	items := convertGameEnvs(envs)
	assert.Len(t, items, 2)
	assert.Equal(t, "prod", items[0].Env)
	assert.Equal(t, "Production", items[0].Description)
	assert.Equal(t, "#ff0000", items[0].Color)
	assert.Equal(t, "dev", items[1].Env)
}

func TestHelpers_BuildGameInfo_Extra(t *testing.T) {
	game := &model.Game{
		Name:        "testgame",
		AliasName:   "Test Game",
		Description: "A test game",
		Status:      "dev",
		Enabled:     true,
	}

	info := buildGameInfo(game)
	assert.Equal(t, "testgame", info.Name)
	assert.Equal(t, "Test Game", info.AliasName)
	assert.Equal(t, "A test game", info.Description)
	assert.Equal(t, "dev", info.Status)
	assert.True(t, info.Enabled)
}

func TestService_Detail_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.Detail(ctx, &GameDetailRequest{ID: "1"})
	assert.Error(t, err)
}

func TestService_Update_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.Update(ctx, &GameUpdateRequest{ID: "1", AliasName: "X"})
	assert.Error(t, err)
}

func TestService_Update_InvalidID_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)
	service := NewService(svcCtx)

	tests := []struct {
		name string
		id   string
	}{
		{"empty id", ""},
		{"non-numeric id", "abc"},
		{"zero id", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Update(ctx, &GameUpdateRequest{ID: tt.id, AliasName: "X"})
			assert.Error(t, err)
		})
	}
}

func TestService_EnvsList_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: "1"})
	assert.Error(t, err)
}

func TestService_EnvAdd_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.EnvAdd(ctx, &GameEnvAddRequest{ID: "1", Name: "prod"})
	assert.Error(t, err)
}

func TestService_EnvUpdate_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: "1", EnvID: "prod", Name: "x"})
	assert.Error(t, err)
}

func TestService_EnvDelete_Unauthorized_Extra(t *testing.T) {
	_, svcCtx := setupGameTestDB(t)
	ctx := context.WithValue(context.Background(), "username", "nobody")
	service := NewService(svcCtx)

	_, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{ID: "1", EnvID: "prod"})
	assert.Error(t, err)
}

func TestService_EnvsList_CorruptedJSON_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "corruptenv", AliasName: "Corrupt", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	// Corrupt the envs JSON directly
	err = db.Exec("UPDATE games SET envs = 'not-valid-json' WHERE id = ?", game.ID).Error
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvsList(ctx, &GameEnvsListRequest{
		ID: strconv.FormatUint(uint64(game.ID), 10),
	})
	assert.Error(t, err)
}

func TestService_EnvAdd_CorruptedJSON_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "corruptadd", AliasName: "CorruptAdd", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	// Corrupt the envs JSON directly
	err = db.Exec("UPDATE games SET envs = 'bad-json' WHERE id = ?", game.ID).Error
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "staging",
	})
	assert.Error(t, err)
}

func TestService_EnvUpdate_CorruptedJSON_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "corruptup", AliasName: "CorruptUp", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	err = db.Exec("UPDATE games SET envs = 'bad-json' WHERE id = ?", game.ID).Error
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "x",
	})
	assert.Error(t, err)
}

func TestService_EnvDelete_CorruptedJSON_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "corruptdel", AliasName: "CorruptDel", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	err = db.Exec("UPDATE games SET envs = 'bad-json' WHERE id = ?", game.ID).Error
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
	})
	assert.Error(t, err)
}

func TestService_Create_WithDescriptionConfig_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:        "desctest",
		AliasName:   "Desc Test",
		Description: "A game with description",
		Config:      `{"key":"value"}`,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "desctest", resp.Game.Name)
	assert.Equal(t, "A game with description", resp.Game.Description)
}

func TestService_Update_Description_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "descgame", AliasName: "Desc", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:          strconv.FormatUint(uint64(game.ID), 10),
		Description: "Updated description",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Updated description", resp.Game.Description)
}

func TestService_Update_Config_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "configgame", AliasName: "Config", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:     strconv.FormatUint(uint64(game.ID), 10),
		Config: `{"setting":"new"}`,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_Update_Name_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "namegame", AliasName: "Name", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "renamedgame",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "renamedgame", resp.Game.Name)
}

func TestService_Update_InvalidName_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "invnamegame", AliasName: "InvName", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	service := NewService(svcCtx)

	_, err = service.Update(ctx, &GameUpdateRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "invalid name!",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "仅支持")
}

func TestService_Delete_NotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	err := service.Delete(ctx, &GameDeleteRequest{ID: "99999"})
	assert.NoError(t, err) // GORM soft delete doesn't error on non-existent
}

func TestService_Detail_InvalidID_Extra2(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.Detail(ctx, &GameDetailRequest{ID: "not_a_number"})
	assert.Error(t, err)
}

func TestService_EnvAdd_GameNotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   "99999",
		Name: "staging",
	})
	assert.Error(t, err)
}

func TestService_EnvUpdate_GameNotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    "99999",
		EnvID: "prod",
	})
	assert.Error(t, err)
}

func TestService_EnvDelete_GameNotFound_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	service := NewService(svcCtx)

	_, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    "99999",
		EnvID: "prod",
	})
	assert.Error(t, err)
}

func TestService_EnvUpdate_OnlyDescription_Extra(t *testing.T) {
	db, svcCtx := setupGameTestDB(t)
	ctx := createGameTestContext(t, db)

	gameModel := svcCtx.GameModel
	game := &model.Game{Name: "envupdesc", AliasName: "EnvUpDesc", Status: "dev"}
	err := gameModel.Create(ctx, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Old"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(ctx, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	service := NewService(svcCtx)

	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Type:  "Updated Production",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
	assert.Equal(t, "Updated Production", resp.Envs[0].Description)
	assert.Equal(t, "prod", resp.Envs[0].Env) // name unchanged
}
