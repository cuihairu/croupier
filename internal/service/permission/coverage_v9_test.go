package permission

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	tokenmgr "github.com/cuihairu/croupier/internal/security/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func requireTableDropV9(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
}

func TestV9_CheckPermission_LoadRolesFails(t *testing.T) {
	db := setupTestDB(t)
	requireTableDropV9(t, db, "roles")

	allowed, err := NewPermissionService(db).CheckPermission(context.Background(), 1, "game", "read")
	require.Error(t, err)
	assert.False(t, allowed)
}

func TestV9_CheckPermission_PluckPermissionIDsFails(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "pluckadmin"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "test_role"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)
	requireTableDropV9(t, db, "role_permissions")

	allowed, err := NewPermissionService(db).CheckPermission(context.Background(), admin.ID, "game", "read")
	require.Error(t, err)
	assert.False(t, allowed)
}

func TestV9_Middleware_GameScope_ExtractGameIDFails(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "scope-no-game"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "player", Action: "create", CheckGameScope: true}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No game ID in path, query, or header.
	req := httptest.NewRequest("GET", "/api/players", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestV9_Middleware_GameScope_DbError(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "scope-db-err"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)
	// admin_game_scopes table intentionally not created -> query error.

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "player", Action: "create", CheckGameScope: true}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10/players", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestV9_Middleware_EnvScope_ExtractGameIDFails(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "env-no-game"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "function", Action: "execute", CheckEnvScope: true}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/functions", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestV9_Middleware_EnvScope_ExtractEnvFails(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "env-missing"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "function", Action: "execute", CheckEnvScope: true}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Game ID present in path but env absent from query and header.
	req := httptest.NewRequest("GET", "/api/games/10/functions", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestV9_Middleware_EnvScope_DbError(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "env-db-err"}
	require.NoError(t, db.Create(&admin).Error)
	role := model.Role{Name: "admin"}
	require.NoError(t, db.Create(&role).Error)
	require.NoError(t, db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", admin.ID, role.ID).Error)
	// admin_game_env_scopes table intentionally not created -> query error.

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "function", Action: "execute", CheckEnvScope: true}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10/functions?env=development", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestV9_Middleware_JWTAdminLookupFails(t *testing.T) {
	db := setupTestDB(t)

	svc := NewPermissionService(db)
	jwtSecret := "v9-test-secret"
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, jwtSecret, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	mgr := tokenmgr.NewManager(jwtSecret)
	token, err := mgr.Sign("ghost-user", []string{"admin"}, 3600000000000)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Valid token but unknown subject -> admin lookup fails with 401.
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestV9_LookupAdminByUsername_TableMissing(t *testing.T) {
	db := setupTestDB(t)
	requireTableDropV9(t, db, "admins")

	_, err := NewPermissionService(db).lookupAdminByUsername("somebody")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lookup admin")
}
