package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAuthorityMiddleware_AllowsAuthorizedRequest(t *testing.T) {
	t.Parallel()

	db := openAuthorityTestDB(t)
	adminID := seedAuthorityAdmin(t, db, "editor", []string{"role:read"}, false)
	mw := NewAuthorityMiddleware(
		permissionservice.NewPermissionService(db),
		"",
		permissionservice.PermissionConfig{Resource: "role", Action: "read"},
	)

	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	req = req.WithContext(context.WithValue(req.Context(), "adminID", adminID))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAuthorityMiddleware_DeniesUnauthorizedRequest(t *testing.T) {
	t.Parallel()

	db := openAuthorityTestDB(t)
	adminID := seedAuthorityAdmin(t, db, "viewer", []string{"game:read"}, false)
	mw := NewAuthorityMiddleware(
		permissionservice.NewPermissionService(db),
		"",
		permissionservice.PermissionConfig{Resource: "role", Action: "read"},
	)

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	req = req.WithContext(context.WithValue(req.Context(), "adminID", adminID))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorityMiddleware_RequiresGameScopeWhenConfigured(t *testing.T) {
	t.Parallel()

	db := openAuthorityTestDB(t)
	adminID := seedAuthorityAdmin(t, db, "scoped", []string{"game:read"}, false)
	mw := NewAuthorityMiddleware(
		permissionservice.NewPermissionService(db),
		"",
		permissionservice.PermissionConfig{Resource: "game", Action: "read", CheckGameScope: true},
	)

	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/games", nil)
	req = req.WithContext(context.WithValue(req.Context(), "adminID", adminID))
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthorityMiddleware_PassthroughWithoutPermissionService(t *testing.T) {
	t.Parallel()

	mw := NewAuthorityMiddleware(nil, "")
	called := false
	handler := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func openAuthorityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func seedAuthorityAdmin(t *testing.T, db *gorm.DB, username string, permissions []string, adminRole bool) uint {
	t.Helper()

	admin := &model.Admin{Username: username, PasswordHash: "hashed-password", Status: 1}
	require.NoError(t, db.Create(admin).Error)

	roleName := "custom"
	if adminRole {
		roleName = "admin"
	}
	role := &model.Role{Name: roleName, Description: "test role"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)

	for _, permissionID := range permissions {
		perm := &model.Permission{ID: permissionID, Name: permissionID, Resource: "test", Action: "read", Category: "test"}
		require.NoError(t, db.Create(perm).Error)
		require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}).Error)
	}

	return admin.ID
}
