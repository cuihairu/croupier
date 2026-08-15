package auth

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- WithGameModel tests (covers 0% → 100%) ---

func TestWithGameModel_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "secret")
	gameModel := model.NewGameModel(db)

	result := service.WithGameModel(gameModel)
	assert.NotNil(t, result)
	assert.Equal(t, service, result)
	assert.NotNil(t, service.gameModel)
}

// --- validLastScope tests (covers 19% → much higher) ---

func TestValidLastScope_EmptyGameID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "secret")

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, "", "prod")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_EmptyEnv(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "secret")

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, "game1", "")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_NilGameModel(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "secret")
	// gameModel is nil

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, "game1", "prod")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_GameNotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, "nonexistent", "prod")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_NoEnvBinding(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	// Create game but no env binding
	game := &model.Game{Name: "testgame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, game.GameID, "prod")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_AdminRole(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	// Create game with env binding
	game := &model.Game{Name: "testgame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	gameID, env := service.validLastScope(context.Background(), 1, []string{"admin"}, game.GameID, "prod")
	assert.Equal(t, game.GameID, gameID)
	assert.Equal(t, "prod", env)
}

func TestValidLastScope_SuperAdminRole(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	game := &model.Game{Name: "testgame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	gameID, env := service.validLastScope(context.Background(), 1, []string{"super_admin"}, game.GameID, "prod")
	assert.Equal(t, game.GameID, gameID)
	assert.Equal(t, "prod", env)
}

func TestValidLastScope_NonAdminWithScope(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	game := &model.Game{Name: "testgame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	// Create admin with viewer role
	admin := &model.Admin{Username: "viewer", Nickname: "Viewer", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))
	role := &model.Role{Name: "viewer"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	// No env scopes set, so should return empty
	gameID, env := service.validLastScope(context.Background(), admin.ID, []string{"viewer"}, game.GameID, "prod")
	assert.Equal(t, "", gameID)
	assert.Equal(t, "", env)
}

func TestValidLastScope_NonAdminWithEnvScope(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	gameModel := model.NewGameModel(db)

	service := NewService(adminModel, permSvc, "secret")
	service.WithGameModel(gameModel)

	game := &model.Game{Name: "testgame", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	// Create admin with viewer role
	admin := &model.Admin{Username: "viewer2", Nickname: "Viewer2", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))
	role := &model.Role{Name: "viewer"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	// Grant env scope
	require.NoError(t, adminModel.SetGameEnvScope(context.Background(), admin.ID, game.ID, "prod"))

	gameID, env := service.validLastScope(context.Background(), admin.ID, []string{"viewer"}, game.GameID, "prod")
	assert.Equal(t, game.GameID, gameID)
	assert.Equal(t, "prod", env)
}

// --- Login with opsStore (covers audit recording branches) ---

func TestService_Login_WithOpsStore_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	tmpDir := t.TempDir()
	store := svc.NewOpsStateStore(tmpDir)

	createTestAdminWithRole(t, db, "audituser", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key", store)

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username:  "audituser",
		Password:  "password123",
		ClientIP:  "10.0.0.1",
		UserAgent: "TestAgent/1.0",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)

	// Verify audit entry was created
	state := store.Snapshot()
	assert.GreaterOrEqual(t, len(state.Audit.Entries), 1)
	lastEntry := state.Audit.Entries[len(state.Audit.Entries)-1]
	assert.Equal(t, "auth.login", lastEntry.Action)
	assert.Equal(t, "audituser", lastEntry.UserID)
	assert.Equal(t, "success", lastEntry.Result)
}

func TestService_Login_FailedPassword_WithOpsStore_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)
	tmpDir := t.TempDir()
	store := svc.NewOpsStateStore(tmpDir)

	createTestAdminWithRole(t, db, "auditfail", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key", store)

	_, err := service.Login(context.Background(), &LoginRequest{
		Username:  "auditfail",
		Password:  "wrongpassword",
		ClientIP:  "10.0.0.1",
		UserAgent: "TestAgent/1.0",
	})
	assert.Error(t, err)

	// Verify audit entry was created for failed login
	state := store.Snapshot()
	assert.GreaterOrEqual(t, len(state.Audit.Entries), 1)
	lastEntry := state.Audit.Entries[len(state.Audit.Entries)-1]
	assert.Equal(t, "auth.login_failed", lastEntry.Action)
	assert.Equal(t, "failed", lastEntry.Result)
}

// --- Logout handler error path (covers 66.7% → 100%) ---

func TestHandler_Logout_ErrorPath_V4(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newAuthTestContext("POST", "/logout", `invalid`)
	handler.Logout(ctx)

	// Logout has no required fields, so even invalid JSON is accepted
	// The service returns empty response, so status should be 200
	assert.Equal(t, 200, rec.Code)
}

// --- Login with whitespace username (covers whitespace handling) ---

func TestService_Login_WhitespaceUsernameV4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "wsuser_auth", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")
	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "  wsuser_auth  ",
		Password: "password123",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
}

// --- Login with whitespace password ---

func TestService_Login_WhitespacePasswordV4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "wspass_auth", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")
	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "wspass_auth",
		Password: "  password123  ",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
}

// --- Check with permission error ---

func TestService_Check_PermissionError_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "chkuser", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")

	resp, err := service.Check(context.Background(), "chkuser", &CheckRequest{
		Resource: "nonexistent_resource",
		Action:   "unknown_action",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.NotEmpty(t, resp.Reason)
}

// --- BatchCheck with multiple checks ---

func TestService_BatchCheck_MultipleV4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "batchuser_v4", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")

	resp, err := service.BatchCheck(context.Background(), "batchuser_v4", &BatchCheckRequest{
		Checks: []CheckRequest{
			{Resource: "game", Action: "read"},
			{Resource: "admin", Action: "write"},
			{Resource: "nonexistent", Action: "delete"},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Results, 3)
}

// --- Check with whitespace in resource/action ---

func TestService_Check_WhitespaceResourceAction_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "wsres", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")

	resp, err := service.Check(context.Background(), "wsres", &CheckRequest{
		Resource: "  game  ",
		Action:   "  read  ",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- recordLoginAudit with empty reason ---

func TestRecordLoginAudit_EmptyReason_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	tmpDir := t.TempDir()
	store := svc.NewOpsStateStore(tmpDir)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret", store)

	service.recordLoginAudit("testuser", "auth.login", "success", &LoginRequest{
		ClientIP:  "127.0.0.1",
		UserAgent: "TestAgent",
	}, "")

	state := store.Snapshot()
	assert.GreaterOrEqual(t, len(state.Audit.Entries), 1)
	lastEntry := state.Audit.Entries[len(state.Audit.Entries)-1]
	assert.Equal(t, "auth.login", lastEntry.Action)
	// No reason in metadata when empty
	_, hasReason := lastEntry.Metadata["reason"]
	assert.False(t, hasReason)
}

// --- recordLoginAudit with nil opsStore ---

func TestRecordLoginAudit_NilOpsStore_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret")

	// Should not panic
	service.recordLoginAudit("testuser", "auth.login", "success", nil, "")
}

// --- recordLoginAudit with nil service ---

func TestRecordLoginAudit_NilService_V4(t *testing.T) {
	t.Parallel()
	var s *Service
	// Should not panic
	s.recordLoginAudit("testuser", "auth.login", "success", nil, "")
}

// --- Login with empty username after trim ---

func TestService_Login_EmptyUsernameAfterTrim_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	_, err := service.Login(context.Background(), &LoginRequest{
		Username: "   ",
		Password: "password123",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "用户名不能为空")
}

// --- Login with empty password after trim ---

func TestService_Login_EmptyPasswordAfterTrim_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	_, err := service.Login(context.Background(), &LoginRequest{
		Username: "testuser",
		Password: "   ",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "密码不能为空")
}

// --- Login with whitespace-only request fields ---

func TestRecordLoginAudit_WhitespaceFields_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	tmpDir := t.TempDir()
	store := svc.NewOpsStateStore(tmpDir)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret", store)

	req := &LoginRequest{
		ClientIP:  "   ",
		UserAgent: "  ",
	}

	service.recordLoginAudit("user", "auth.login", "success", req, "")

	state := store.Snapshot()
	assert.GreaterOrEqual(t, len(state.Audit.Entries), 1)
	lastEntry := state.Audit.Entries[len(state.Audit.Entries)-1]
	_, hasIP := lastEntry.Metadata["ip"]
	_, hasUA := lastEntry.Metadata["user_agent"]
	assert.False(t, hasIP)
	assert.False(t, hasUA)
}

// --- recordLoginAudit with large entry count (audit trimming) ---

func TestRecordLoginAudit_AuditTrimming_V4(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	tmpDir := t.TempDir()
	store := svc.NewOpsStateStore(tmpDir)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "secret", store)

	// Pre-populate with 2001 entries
	store.Update(func(st *svc.OpsState) {
		for i := 0; i < 2001; i++ {
			st.Audit.Entries = append(st.Audit.Entries, svc.OpsAuditEntry{
				ID:        "old-" + string(rune('a'+i%26)),
				Action:    "old.action",
				UserID:    "olduser",
				Result:    "success",
				CreatedAt: time.Now(),
			})
		}
	})

	service.recordLoginAudit("newuser", "auth.login", "success", nil, "")

	state := store.Snapshot()
	assert.LessOrEqual(t, len(state.Audit.Entries), 2000)
	lastEntry := state.Audit.Entries[len(state.Audit.Entries)-1]
	assert.Equal(t, "newuser", lastEntry.UserID)
}
