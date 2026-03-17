package assignment

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	integrationDB     *gorm.DB
	integrationSvcCtx *svc.ServiceContext
	integrationOnce   sync.Once
	testCounter       int64
)

// setupIntegrationTestDB creates a test database with all required tables (once)
func setupIntegrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	integrationOnce.Do(func() {
		var err error
		integrationDB, err = gorm.Open(gsqlite.Open("file:integration_test.db?mode=memory&cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			panic(err)
		}

		// Migrate all required models
		err = integrationDB.AutoMigrate(
			&model.Admin{},
			&model.AdminRole{},
			&model.Role{},
			&model.RolePermission{},
			&model.Permission{},
		)
		if err != nil {
			panic(err)
		}

		// Set up service context
		tmpDir := t.TempDir()
		assignmentsFile := filepath.Join(tmpDir, "assignments.json")

		integrationSvcCtx = &svc.ServiceContext{
			DB: integrationDB,
			Config: config.Config{
				BootstrapData: config.BootstrapDataConfig{
					BaseDir: tmpDir,
				},
				Registry: config.RegistryConfig{
					AssignmentsPath: assignmentsFile,
				},
			},
			RegistryStore:   registry.NewStore(),
			AdminModel:      model.NewAdminModel(integrationDB),
			RoleModel:       model.NewRoleModel(integrationDB),
			PermissionModel: model.NewPermissionModel(integrationDB),
		}
	})

	return integrationDB
}

// createTestAdmin creates a test admin with role for the current test
func createTestAdmin(t *testing.T, db *gorm.DB) (string, string) {
	t.Helper()
	testCounter++

	// Create unique permissions
	permWriteID := fmt.Sprintf("perm_write_%d", testCounter)
	permWrite := &model.Permission{
		ID:       permWriteID,
		Name:     fmt.Sprintf("assignments:write:%d", testCounter),
		Resource: "assignments",
		Action:   "write",
		Category: "assignments",
	}
	err := db.Create(permWrite).Error
	require.NoError(t, err)

	permAllID := "admin:all" // Keep this as the standard admin permission ID
	permAll := &model.Permission{
		ID:       permAllID,
		Name:     "admin:all",
		Resource: "*",
		Action:   "*",
		Category: "admin",
	}
	// Only create admin:all if it doesn't exist
	var existingPerm model.Permission
	err = db.Where("id = ?", permAllID).First(&existingPerm).Error
	if err == gorm.ErrRecordNotFound {
		err = db.Create(permAll).Error
		require.NoError(t, err)
	}

	// Create role
	role := &model.Role{
		Name: fmt.Sprintf("admin_role_%d", testCounter),
	}
	err = db.Create(role).Error
	require.NoError(t, err)

	// Use RoleModel.ReplacePermissions to assign permissions
	ctx := context.Background()
	err = integrationSvcCtx.RoleModel.ReplacePermissions(ctx, role.ID, []string{permAllID, permWriteID})
	require.NoError(t, err)

	// Create admin
	username := fmt.Sprintf("testadmin_%d", testCounter)
	admin := &model.Admin{
		Username:     username,
		Nickname:     "Test Admin",
		Email:        fmt.Sprintf("test%d@example.com", testCounter),
		PasswordHash: "$2a$10$hash",
		Status:       1,
	}
	err = db.Create(admin).Error
	require.NoError(t, err)

	// Create admin role association
	err = db.Create(&model.AdminRole{
		AdminID: admin.ID,
		RoleID:  role.ID,
	}).Error
	require.NoError(t, err)

	return username, permAllID
}

// TestAssignmentsUpdate_Integration_Success tests the full update flow with auth
func TestAssignmentsUpdate_Integration_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    fmt.Sprintf("game_success_%d", testCounter),
		Env:       "dev",
		Functions: []string{"func1", "func2", "func3"},
	}

	resp, err := logic.AssignmentsUpdate(req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data, "ok")
	assert.True(t, data["ok"].(bool))
}

// TestAssignmentsUpdate_Integration_UpdateExisting tests updating existing assignments
func TestAssignmentsUpdate_Integration_UpdateExisting(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)
	gameID := fmt.Sprintf("game_update_%d", testCounter)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	// First, create initial assignments
	req1 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "dev",
		Functions: []string{"func1", "func2"},
	}

	_, err := logic.AssignmentsUpdate(req1)
	require.NoError(t, err)

	// Now update with different functions
	req2 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "dev",
		Functions: []string{"func2", "func3", "func4"},
		Action:    "update",
	}

	resp2, err := logic.AssignmentsUpdate(req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)

	data, ok := resp2.Data.(map[string]interface{})
	require.True(t, ok)

	assignments, ok := data["assignments"].(map[string][]string)
	require.True(t, ok)
	key := gameID + "|dev"
	assert.Contains(t, assignments, key)
	assert.Len(t, assignments[key], 3)
}

// TestAssignmentsUpdate_Integration_RemoveAll tests removing all functions
func TestAssignmentsUpdate_Integration_RemoveAll(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)
	gameID := fmt.Sprintf("game_remove_%d", testCounter)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	// First, create assignments
	req1 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "dev",
		Functions: []string{"func1", "func2"},
	}

	_, err := logic.AssignmentsUpdate(req1)
	require.NoError(t, err)

	// Now remove all
	req2 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "dev",
		Functions: []string{},
		Action:    "remove",
	}

	resp2, err := logic.AssignmentsUpdate(req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)
}

// TestAssignmentsUpdate_Integration_HistoryTracked tests that history is recorded
func TestAssignmentsUpdate_Integration_HistoryTracked(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)
	gameID := fmt.Sprintf("game_history_%d", testCounter)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "test",
		Functions: []string{"func1"},
		Action:    "assign",
	}

	_, err := logic.AssignmentsUpdate(req)
	require.NoError(t, err)

	// Give some time for async history write
	time.Sleep(10 * time.Millisecond)

	// Verify history file was created
	historyPath := assignmentHistoryPath(integrationSvcCtx)
	historyEntries, err := loadAssignmentHistory(historyPath)
	require.NoError(t, err)
	assert.NotEmpty(t, historyEntries)

	// Find our entry
	var found *assignmentHistoryEntry
	for _, entry := range historyEntries {
		if entry.GameID == gameID && entry.Env == "test" {
			found = &entry
			break
		}
	}
	assert.NotNil(t, found)
	assert.Equal(t, username, found.OperatedBy)
}

// TestAssignmentsUpdate_Integration_WhitespaceNormalization tests input normalization
func TestAssignmentsUpdate_Integration_WhitespaceNormalization(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "  game6  ",
		Env:       "  dev  ",
		Functions: []string{"  func1  ", "  func2  ", "", "  "},
	}

	resp, err := logic.AssignmentsUpdate(req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	assignments, ok := data["assignments"].(map[string][]string)
	require.True(t, ok)

	// Game ID and env should be normalized
	key := "game6|dev"
	assert.Contains(t, assignments, key)
	// Empty/whitespace functions should be filtered
	assert.Len(t, assignments[key], 2)
}

// TestAssignmentsUpdate_Integration_InvalidGameId tests empty game ID
func TestAssignmentsUpdate_Integration_InvalidGameId(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "",
		Env:       "dev",
		Functions: []string{"func1"},
	}

	resp, err := logic.AssignmentsUpdate(req)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// TestAssignmentsUpdate_Integration_DuplicateFunctions tests handling duplicate functions
func TestAssignmentsUpdate_Integration_DuplicateFunctions(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    fmt.Sprintf("game_dup_%d", testCounter),
		Env:       "dev",
		Functions: []string{"func1", "func1", "func2", "func2", "func3"},
	}

	resp, err := logic.AssignmentsUpdate(req)
	require.NoError(t, err)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	assignments, ok := data["assignments"].(map[string][]string)
	require.True(t, ok)

	key := fmt.Sprintf("game_dup_%d|dev", testCounter)
	assert.Len(t, assignments[key], 3) // Duplicates should be removed
}

// TestAssignmentsUpdate_Integration_MultipleEnvironments tests same game different envs
func TestAssignmentsUpdate_Integration_MultipleEnvironments(t *testing.T) {
	db := setupIntegrationTestDB(t)
	username, _ := createTestAdmin(t, db)
	gameID := fmt.Sprintf("game_multi_env_%d", testCounter)

	ctx := context.WithValue(context.Background(), "username", username)
	logic := NewAssignmentsUpdateLogic(ctx, integrationSvcCtx)

	// Create assignments for dev environment
	req1 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "dev",
		Functions: []string{"func1", "func2"},
	}

	_, err := logic.AssignmentsUpdate(req1)
	require.NoError(t, err)

	// Create assignments for prod environment
	req2 := &AssignmentsUpdateRequest{
		GameId:    gameID,
		Env:       "prod",
		Functions: []string{"func3", "func4"},
	}

	_, err = logic.AssignmentsUpdate(req2)
	require.NoError(t, err)

	// Verify both environments exist
	path := assignmentsPath(integrationSvcCtx)
	assignments, err := loadAssignments(path)
	require.NoError(t, err)

	assert.Contains(t, assignments, gameID+"|dev")
	assert.Contains(t, assignments, gameID+"|prod")
}
