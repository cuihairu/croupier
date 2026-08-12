package assignment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAssignmentTestDB(t *testing.T) (*gorm.DB, *svc.ServiceContext) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create test admin
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

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
		"admin:all", "assignments:read", "assignments:write",
	})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "assignments:read", Name: "Assignments Read", Resource: "assignments", Action: "read", Category: "assignment"},
		{ID: "assignments:write", Name: "Assignments Write", Resource: "assignments", Action: "write", Category: "assignment"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	tmpDir := t.TempDir()

	svcCtx := &svc.ServiceContext{
		DB:                db,
		AdminModel:        adminModel,
		GameModel:         model.NewGameModel(db),
		RoleModel:         roleModel,
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: filepath.Join(tmpDir, "assignments.json"),
			},
		},
	}

	return db, svcCtx
}

func TestService_Update_CloneUsesScopedSourceAndExplicitTarget(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)
	ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "game1", Env: "prod"})

	game := &model.Game{GameID: "game1", Name: "Game One"}
	require.NoError(t, svcCtx.GameModel.Create(ctx, game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "game1", "prod", "game1_prod", "", ""))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "game1", "stage", "game1_stage", "", ""))

	service := NewService(svcCtx)
	resp, err := service.Update(ctx, &AssignmentsUpdateRequest{
		GameId:    "forbidden-game",
		Env:       "dev",
		Action:    "clone",
		TargetEnv: "stage",
		Functions: []string{"func1"},
	})
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.Equal(t, []string{"func1"}, resp.Assignments["game1|stage"])
	assert.NotContains(t, resp.Assignments, "forbidden-game|dev")

	assignments, err := loadAssignments(svcCtx.Config.Registry.AssignmentsPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"func1"}, assignments["game1|stage"])
	assert.NotContains(t, assignments, "game1|prod")
}

func createAssignmentTestContext(t *testing.T, db *gorm.DB) context.Context {
	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)
	return ctx
}

func TestService_List_WithAdmin_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create test data
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	testData := map[string][]string{
		"game1|prod": {"func1", "func2"},
		"game2|dev":  {"func3"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsListRequest{
		GameId: "game1",
	}

	resp, err := service.List(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_List_FilterByEnv_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create test data
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	testData := map[string][]string{
		"game1|prod": {"func1"},
		"game1|dev":  {"func2"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsListRequest{
		GameId: "game1",
		Env:    "prod",
	}

	resp, err := service.List(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_List_EmptyFile_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	service := NewService(svcCtx)

	req := &AssignmentsListRequest{}

	resp, err := service.List(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Total)
}

func TestService_History_WithAdmin_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create test history
	historyPath := assignmentHistoryPath(svcCtx)
	testHistory := []assignmentHistoryEntry{
		{ID: "1", GameID: "game1", Env: "prod", FunctionID: "func1", Action: "add"},
		{ID: "2", GameID: "game1", Env: "dev", FunctionID: "func2", Action: "add"},
	}
	err := saveAssignmentHistory(historyPath, testHistory)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsHistoryRequest{
		GameId: "game1",
		Env:    "prod",
	}

	resp, err := service.History(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_History_Pagination_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create test history with 25 entries
	historyPath := assignmentHistoryPath(svcCtx)
	testHistory := make([]assignmentHistoryEntry, 25)
	for i := 0; i < 25; i++ {
		testHistory[i] = assignmentHistoryEntry{
			ID:         string(rune(i)),
			GameID:     "game1",
			FunctionID: "func1",
			Action:     "add",
		}
	}
	err := saveAssignmentHistory(historyPath, testHistory)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsHistoryRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := service.History(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 10, len(resp.Items))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PageSize)
}

func TestService_History_FilterByAction_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create test history
	historyPath := assignmentHistoryPath(svcCtx)
	testHistory := []assignmentHistoryEntry{
		{ID: "1", GameID: "game1", Env: "prod", FunctionID: "func1", Action: "add"},
		{ID: "2", GameID: "game1", Env: "prod", FunctionID: "func2", Action: "remove"},
	}
	err := saveAssignmentHistory(historyPath, testHistory)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsHistoryRequest{
		GameId: "game1",
		Action: "add",
	}

	resp, err := service.History(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
}

func TestService_Update_WithAdmin_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create initial data
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	testData := map[string][]string{
		"game1|prod": {"func1"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "assign",
		Functions: []string{"func1", "func2"},
	}

	resp, err := service.Update(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.OK)
}

func TestService_Update_EmptyGameId_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	service := NewService(svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "",
		Env:       "prod",
		Functions: []string{"func1"},
	}

	_, err := service.Update(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "game_id不能为空")
}

func TestService_Update_RemoveAll_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Create initial data
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	testData := map[string][]string{
		"game1|prod": {"func1", "func2"},
	}
	err := saveAssignments(assignmentsPath, testData)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{},
	}

	resp, err := service.Update(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.OK)

	// Verify assignments were removed (empty slice or nil)
	loaded, err := loadAssignments(assignmentsPath)
	require.NoError(t, err)
	assert.Empty(t, loaded["game1|prod"])
}

func TestService_Update_WithWhitespace_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	service := NewService(svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "  game1  ",
		Env:       "  prod  ",
		Functions: []string{"  func1  ", " func2 ", ""},
	}

	resp, err := service.Update(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.OK)

	// Verify key was normalized
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	loaded, err := loadAssignments(assignmentsPath)
	require.NoError(t, err)
	assert.Contains(t, loaded, "game1|prod")
	assert.Equal(t, []string{"func1", "func2"}, loaded["game1|prod"])
}

func TestService_Update_CreateHistoryEntry_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	service := NewService(svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{"func1"},
	}

	_, err := service.Update(ctx, req)
	assert.NoError(t, err)

	// Verify history file was created
	historyPath := assignmentHistoryPath(svcCtx)
	_, err = os.Stat(historyPath)
	assert.NoError(t, err)
}

func TestService_List_InvalidJSON_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Write invalid JSON
	assignmentsPath := svcCtx.Config.Registry.AssignmentsPath
	err := os.WriteFile(assignmentsPath, []byte("{ invalid json"), 0644)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsListRequest{}

	_, err = service.List(ctx, req)
	assert.Error(t, err)
}

func TestService_History_InvalidJSON_Extra(t *testing.T) {
	db, svcCtx := setupAssignmentTestDB(t)
	ctx := createAssignmentTestContext(t, db)

	// Write invalid JSON
	historyPath := assignmentHistoryPath(svcCtx)
	err := os.WriteFile(historyPath, []byte("[ invalid json"), 0644)
	require.NoError(t, err)

	service := NewService(svcCtx)

	req := &AssignmentsHistoryRequest{}

	_, err = service.History(ctx, req)
	assert.Error(t, err)
}
