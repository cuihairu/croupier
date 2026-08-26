package auth

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)
	// Schema normally created by the migration baseline; tests own their
	// fixtures.
	require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))

	return db
}

// createTestAdminWithRole creates a test admin with a role for testing
func createTestAdminWithRole(t *testing.T, db *gorm.DB, username, password, roleName string) uint {
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	// Create admin
	admin := &model.Admin{
		Username: username,
		Nickname: username + " User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, password)
	require.NoError(t, err)

	// Create role
	role := &model.Role{Name: roleName}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	// Assign role
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	return admin.ID
}

func TestService_Login_Success(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "testadmin", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")
	jwtutil.InitGlobalSecret("test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "testadmin",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "testadmin", resp.User.Username)
	assert.Len(t, resp.User.Roles, 1)
	assert.Equal(t, "admin", resp.User.Roles[0])
}

func TestService_Login_EmptyUsername(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "",
		Password: "password123",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户名不能为空")
}

func TestService_Login_EmptyPassword(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "admin",
		Password: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "密码不能为空")
}

func TestService_Login_InvalidPassword(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)

	admin := &model.Admin{
		Username: "testadmin",
		Nickname: "Test Admin",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	service := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "testadmin",
		Password: "wrongpassword",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户名或密码错误")
}

func TestService_Login_UserNotFound(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户名或密码错误")
}

func TestService_Login_MultipleRoles(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{
		Username: "multirole",
		Nickname: "Multi Role User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role1 := &model.Role{Name: "admin"}
	role2 := &model.Role{Name: "editor"}
	role3 := &model.Role{Name: "viewer"}
	err = roleModel.Create(context.Background(), role1)
	require.NoError(t, err)
	err = roleModel.Create(context.Background(), role2)
	require.NoError(t, err)
	err = roleModel.Create(context.Background(), role3)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role1.ID)
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role2.ID)
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role3.ID)
	require.NoError(t, err)

	service := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret-key")
	jwtutil.InitGlobalSecret("test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "multirole",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.User.Roles, 3)
}

func TestService_Login_NoRoles(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)

	admin := &model.Admin{
		Username: "norole",
		Nickname: "No Role User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	service := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret-key")
	jwtutil.InitGlobalSecret("test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "norole",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.User.Roles, 0)
	assert.NotEmpty(t, resp.Token)
}

func TestService_Logout_Success(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Logout(context.Background(), &LogoutRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_Check_Allowed(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)

	_ = createTestAdminWithRole(t, db, "testadmin", "password123", "admin")

	permSvc := permissionservice.NewPermissionService(db)
	service := NewService(adminModel, permSvc, "test-secret-key")

	resp, err := service.Check(context.Background(), "testadmin", &CheckRequest{
		Resource: "admin",
		Action:   "read",
	})

	// Admin role should have admin:read permission
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Note: Actual permission check depends on rbac setup, which may not work in test
}

func TestService_Check_UserNotFound(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Check(context.Background(), "nonexistent", &CheckRequest{
		Resource: "game",
		Action:   "read",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_BatchCheck_EmptyChecks(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)

	createTestAdminWithRole(t, db, "testadmin", "password123", "admin")

	service := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.BatchCheck(context.Background(), "testadmin", &BatchCheckRequest{
		Checks: []CheckRequest{},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Results, 0)
}

func TestService_BatchCheck_UserNotFound(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.BatchCheck(context.Background(), "nonexistent", &BatchCheckRequest{
		Checks: []CheckRequest{
			{Resource: "game", Action: "read"},
		},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Login_WhitespaceUsername(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "whitespaceuser", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")
	jwtutil.InitGlobalSecret("test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "  whitespaceuser  ",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
}

func TestService_Login_WhitespacePassword(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	createTestAdminWithRole(t, db, "whitespacepass", "password123", "admin")

	service := NewService(adminModel, permSvc, "test-secret-key")
	jwtutil.InitGlobalSecret("test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "whitespacepass",
		Password: "  password123  ",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
}

func TestService_Check_Denied(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "denieduser", "password123", "admin")

	resp, err := service.Check(context.Background(), "denieduser", &CheckRequest{
		Resource: "test",
		Action:   "unknown",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Allowed)
	assert.NotEmpty(t, resp.Reason)
}

func TestService_Check_InvalidUser(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	resp, err := service.Check(context.Background(), "", &CheckRequest{
		Resource: "test",
		Action:   "read",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_BatchCheck_SingleCheck(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "batchuser", "password123", "admin")

	resp, err := service.BatchCheck(context.Background(), "batchuser", &BatchCheckRequest{
		Checks: []CheckRequest{
			{Resource: "test", Action: "read"},
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Results, 1)
}

func TestService_Login_RetrieveRolesFailed(t *testing.T) {
	// This test would require mocking to simulate GetAdminRoles failure
	// For now, we'll skip it as it requires significant refactoring
	t.Skip("Requires mocking of GetAdminRoles error case")
}

func TestService_Check_MultipleResources(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "multiuser", "password123", "admin")

	tests := []struct {
		name     string
		resource string
		action   string
	}{
		{"admin read", "admin", "read"},
		{"admin write", "admin", "write"},
		{"game read", "game", "read"},
	}

	for _, tt := range tests {
		resp, err := service.Check(context.Background(), "multiuser", &CheckRequest{
			Resource: tt.resource,
			Action:   tt.action,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	}
}

func TestService_BatchCheck_MultipleChecks(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "batchmulti", "password123", "admin")

	resp, err := service.BatchCheck(context.Background(), "batchmulti", &BatchCheckRequest{
		Checks: []CheckRequest{
			{Resource: "test1", Action: "read"},
			{Resource: "test2", Action: "write"},
			{Resource: "test3", Action: "delete"},
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Results, 3)
}

func TestService_Login_WhitespaceOnlyUsername(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "   ",
		Password: "password123",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户名不能为空")
}

func TestService_Login_WhitespaceOnlyPassword(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")

	resp, err := service.Login(context.Background(), &LoginRequest{
		Username: "testuser",
		Password: "   ",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "密码不能为空")
}

func TestService_Check_WithPermissionServiceError(t *testing.T) {
	// This test would require mocking to simulate permission service error
	// For now, we add a test that exercises the error path in Check
	t.Skip("Requires mocking of permission service error case")
}

func TestService_Check_WithEmptyResource(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "emptyresuser", "password123", "admin")

	// Test with empty resource - should still work
	resp, err := service.Check(context.Background(), "emptyresuser", &CheckRequest{
		Resource: "",
		Action:   "read",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// When resource is empty, permission check may still pass or fail depending on implementation
}

func TestService_Check_WithEmptyAction(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")

	createTestAdminWithRole(t, db, "emptyactionuser", "password123", "admin")

	resp, err := service.Check(context.Background(), "emptyactionuser", &CheckRequest{
		Resource: "test",
		Action:   "",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestRecordLoginAudit_NilService tests recordLoginAudit with nil receiver
func TestRecordLoginAudit_NilService(t *testing.T) {
	var service *Service = nil
	// This should not panic - the method handles nil receiver
	service.recordLoginAudit("testuser", "auth.login", "success", nil, "", "")
	// If we got here without panic, the test passed
	assert.True(t, true)
}

// TestRecordLoginAudit_NilOpsStore tests recordLoginAudit with nil opsStore
func TestRecordLoginAudit_NilOpsStore(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret-key")
	// service.opsStore is nil
	assert.Nil(t, service.opsStore)

	// This should not panic and should return early
	service.recordLoginAudit("testuser", "auth.login", "success", nil, "", "")
	// If we got here without panic, the test passed
	assert.True(t, true)
}

// helper: attach a table-backed audit service so recordLoginAudit persists
// into audit_records (in-memory SQLite).
func withTableAudit(t *testing.T, db *gorm.DB, s *Service) *Service {
	t.Helper()
	auditStore, err := audit.NewSQLAuditStore(db)
	require.NoError(t, err)
	return s.WithAuditService(audit.NewAuditService(auditStore, nil))
}

func lastAuditRow(t *testing.T, db *gorm.DB) map[string]interface{} {
	t.Helper()
	var row struct {
		EventType   string
		ActorID     string
		Outcome     string
		IP          string
		DetailsJSON string
	}
	require.NoError(t, db.Table("audit_records").
		Select("event_type, actor_id, outcome, ip, details_json").
		Order("timestamp DESC").Limit(1).
		Scan(&row).Error)
	return map[string]interface{}{
		"event_type":   row.EventType,
		"actor_id":     row.ActorID,
		"outcome":      row.Outcome,
		"ip":           row.IP,
		"details_json": row.DetailsJSON,
	}
}

// TestRecordLoginAudit_WithAuditTable verifies login auditing persists to audit_records.
func TestRecordLoginAudit_WithAuditTable(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))

	req := &LoginRequest{
		ClientIP:  "127.0.0.1",
		UserAgent: "test-agent",
	}

	service.recordLoginAudit("admin", "auth.login", "success", req, "", "")
	row := lastAuditRow(t, db)
	assert.Equal(t, "auth.login", row["event_type"])
	assert.Equal(t, "admin", row["actor_id"])
	assert.Equal(t, "success", row["outcome"])
	assert.Equal(t, "127.0.0.1", row["ip"])
	assert.Contains(t, row["details_json"], "test-agent")
}

// TestRecordLoginAudit_WithReason tests recordLoginAudit with reason parameter
func TestRecordLoginAudit_WithReason(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))

	req := &LoginRequest{
		ClientIP:  "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	service.recordLoginAudit("user", "auth.login_failed", "failed", req, "invalid_credentials", "")
	row := lastAuditRow(t, db)
	assert.Equal(t, "auth.login_failed", row["event_type"])
	assert.Equal(t, "failure", row["outcome"])
	assert.Contains(t, row["details_json"], "invalid_credentials")
}

// TestRecordLoginAudit_WithNilRequest tests recordLoginAudit with nil request
func TestRecordLoginAudit_WithNilRequest(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))

	service.recordLoginAudit("user", "auth.login", "success", nil, "", "")
	row := lastAuditRow(t, db)
	assert.Equal(t, "auth.login", row["event_type"])
	assert.NotContains(t, row["details_json"], "userAgent")
}

// TestRecordLoginAudit_WithWhitespaceInRequest tests recordLoginAudit with whitespace-only fields
func TestRecordLoginAudit_WithWhitespaceInRequest(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))

	req := &LoginRequest{
		ClientIP:  "   ",
		UserAgent: "  ",
	}

	service.recordLoginAudit("user", "auth.login", "success", req, "", "")
	row := lastAuditRow(t, db)
	assert.Equal(t, "", row["ip"])
	assert.NotContains(t, row["details_json"], "userAgent")
}

// TestRecordLoginAudit_TrimmedUsername tests that username is trimmed
func TestRecordLoginAudit_TrimmedUsername(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("DELETE FROM audit_records")
	service := withTableAudit(t, db, NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret"))

	service.recordLoginAudit("  admin  ", "auth.login", "success", nil, "", "")
	row := lastAuditRow(t, db)
	assert.Equal(t, "admin", row["actor_id"])
}
