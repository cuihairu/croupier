package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitDatabase_NilConfig(t *testing.T) {
	t.Parallel()

	db, err := initDatabase(nil)
	require.Error(t, err)
	assert.Nil(t, db)
	assert.ErrorIs(t, err, gorm.ErrInvalidDB)
}

func TestInitDatabase_UsesConfiguredDataSource(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			DataSource: "file::memory:?mode=memory",
		},
	}

	db, err := initDatabase(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	assert.NoError(t, sqlDB.Ping())
}

func TestRegisterRoutes_HealthEndpointAvailable(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			DataSource: "file::memory:?mode=memory",
		},
	}

	err := RegisterRoutes(engine, cfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestRegisterRoutes_ProtectedRoleAndPermissionEndpointsMounted(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		Auth:   config.AuthConfig{JWTSecret: "test-router-secret"},
		Database: config.DatabaseConfig{
			DataSource: "file::memory:?mode=memory",
		},
	}

	err := RegisterRoutes(engine, cfg)
	require.NoError(t, err)

	db, err := initDatabase(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, model.AutoMigrate(db))

	adminID := seedRouterAdminWithRoleRead(t, db)
	token, err := jwtutil.Sign("test-router-secret", "router-admin", []string{"admin"}, adminID, 0, nowForTest())
	require.NoError(t, err)

	for _, path := range []string{
		"/api/v1/roles",
		"/api/v1/permissions",
		"/api/v1/games",
		"/api/v1/admins",
		"/api/v1/players",
		"/api/v1/tasks",
		"/api/v1/registry",
		"/api/v1/functions",
		"/api/v1/configs",
		"/api/v1/configs/router-test",
		"/api/v1/configs/router-test/versions",
		"/api/v1/extensions/catalog",
		"/api/v1/extensions/installations",
		"/api/v1/ops/agents",
		"/api/v1/ops/metrics",
		"/api/v1/ops/health",
		"/api/v1/ops/maintenance",
		"/api/v1/ops/config",
		"/api/v1/ops/notifications",
		"/api/v1/ops/services",
		"/api/v1/ops/functions",
		"/api/v1/ops/mq",
		"/api/v1/monitoring/healthz",
		"/api/v1/openapi/document",
		"/api/v1/providers",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusNotFound, rec.Code, path)
	}
}

func TestRegisterRoutes_ReturnsDatabaseError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			DataSource: "postgres://invalid:://dsn",
		},
	}

	err := RegisterRoutes(engine, cfg)
	require.Error(t, err)
}

func seedRouterAdminWithRoleRead(t *testing.T, db *gorm.DB) uint {
	t.Helper()

	admin := &model.Admin{
		Username:     "router-admin",
		PasswordHash: "hashed",
		Status:       1,
	}
	require.NoError(t, db.Create(admin).Error)

	role := &model.Role{
		Name:        "admin",
		Description: "router test role",
	}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)

	perm := &model.Permission{
		ID:          "role:read",
		Name:        "role:read",
		Resource:    "role",
		Action:      "read",
		Category:    "role",
		Description: "read roles",
	}
	require.NoError(t, db.Create(perm).Error)
	require.NoError(t, db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}).Error)

	return admin.ID
}

func nowForTest() (ts time.Time) {
	return time.Now().UTC()
}
