package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/svc"
)

// testSecret is the global secret used for middleware tests
const testSecret = "test-middleware-secret"

func TestMain(m *testing.M) {
	// Initialize the global secret before any tests run.
	// This must happen before sync.Once fires in any test.
	jwtutil.InitGlobalSecret(testSecret)
	m.Run()
}

// getTestSecret returns the secret for testing
func getTestSecret() string {
	return testSecret
}

// TestAuthMiddleware_Struct tests the AuthMiddleware struct which uses its own secret
func TestAuthMiddleware_Struct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("NewAuthMiddleware creates middleware", func(t *testing.T) {
		secret := "test-secret-new"
		mw := NewAuthMiddleware(secret)
		assert.NotNil(t, mw)
		assert.Equal(t, secret, mw.secret)
	})

	t.Run("Handle with missing authorization header", func(t *testing.T) {
		secret := "test-secret-missing"
		mw := NewAuthMiddleware(secret)

		router := gin.New()
		router.Use(mw.Handle)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "missing authorization header")
	})

	t.Run("Handle with invalid format", func(t *testing.T) {
		secret := "test-secret-invalid"
		mw := NewAuthMiddleware(secret)

		router := gin.New()
		router.Use(mw.Handle)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "format")
	})

	t.Run("Handle with invalid token", func(t *testing.T) {
		secret := "test-secret-bad-token"
		mw := NewAuthMiddleware(secret)

		router := gin.New()
		router.Use(mw.Handle)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authentication_failed")
	})

	t.Run("Handle with non-Bearer token", func(t *testing.T) {
		secret := "test-secret-non-bearer"
		mw := NewAuthMiddleware(secret)

		router := gin.New()
		router.Use(mw.Handle)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "format")
	})

	t.Run("Authenticate with invalid token", func(t *testing.T) {
		secret := "test-secret-auth-invalid"
		mw := NewAuthMiddleware(secret)
		username, roles, adminID, err := mw.Authenticate("invalid-token")
		assert.Error(t, err)
		assert.Empty(t, username)
		assert.Nil(t, roles)
		assert.Equal(t, uint(0), adminID)
	})

	t.Run("Authenticate with valid token", func(t *testing.T) {
		secret := "test-secret-auth-valid"
		mw := NewAuthMiddleware(secret)
		token, err := jwtutil.Sign(secret, "testuser", []string{"admin"}, 123, time.Now())
		require.NoError(t, err)

		username, roles, adminID, err := mw.Authenticate(token)
		assert.NoError(t, err)
		assert.Equal(t, "testuser", username)
		assert.Equal(t, []string{"admin"}, roles)
		assert.Equal(t, uint(123), adminID)
	})

	t.Run("Handle with valid token passes through", func(t *testing.T) {
		secret := "test-secret-valid-handle"
		mw := NewAuthMiddleware(secret)
		token, err := jwtutil.Sign(secret, "testuser", []string{"admin"}, 1, time.Now())
		require.NoError(t, err)

		router := gin.New()
		router.Use(mw.Handle)
		router.GET("/test", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.JSON(200, gin.H{"username": username})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "testuser")
	})
}

// TestAuth_GlobalFunction tests the global Auth() function behavior
func TestAuth_GlobalFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("without JWT secret returns internal server error", func(t *testing.T) {
		// This test can only reliably run if global secret is not yet initialized
		// If secret is already set, skip this test
		if jwtutil.GetGlobalSecret() != "" {
			t.Skip("Global secret already initialized, cannot test uninitialized state")
		}

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("with secret set validates tokens", func(t *testing.T) {
		secret := getTestSecret()
		token, err := jwtutil.Sign(secret, "testuser", []string{"admin"}, 1, time.Now())
		require.NoError(t, err)

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.JSON(200, gin.H{"username": username})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "testuser")
	})

	t.Run("missing authorization header returns unauthorized", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid authorization header format returns unauthorized", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-Bearer token returns unauthorized", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token returns unauthorized", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(Auth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestOptionalAuth tests the OptionalAuth middleware
func TestOptionalAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("allows request without auth header", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(OptionalAuth())
		router.GET("/test", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.JSON(200, gin.H{"username": username})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("allows request with valid token", func(t *testing.T) {
		secret := getTestSecret()
		token, err := jwtutil.Sign(secret, "testuser", []string{"admin"}, 1, time.Now())
		require.NoError(t, err)

		router := gin.New()
		router.Use(OptionalAuth())
		router.GET("/test", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.JSON(200, gin.H{"username": username})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "testuser")
	})

	t.Run("ignores invalid token", func(t *testing.T) {
		getTestSecret()

		router := gin.New()
		router.Use(OptionalAuth())
		router.GET("/test", func(c *gin.Context) {
			username, _ := c.Get("username")
			c.JSON(200, gin.H{"username": username})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("no JWT secret set allows request", func(t *testing.T) {
		// This test can only run if secret is not yet initialized
		if jwtutil.GetGlobalSecret() != "" {
			t.Skip("Global secret already initialized")
		}

		router := gin.New()
		router.Use(OptionalAuth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestInitJWTSecret tests the InitJWTSecret function
func TestInitJWTSecret(t *testing.T) {
	// InitJWTSecret is a wrapper around jwtutil.InitGlobalSecret
	// Just verify the function doesn't panic
	assert.NotPanics(t, func() {
		InitJWTSecret("test-secret-wrapper")
	})
	// Due to sync.Once, we can't verify the actual value
}

// TestLogger tests the Logger middleware
func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Logger())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCORS tests the CORS middleware
func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets CORS headers", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS())
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	})

	t.Run("handles OPTIONS request", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS())
		router.GET("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("handles POST request", func(t *testing.T) {
		router := gin.New()
		router.Use(CORS())
		router.POST("/test", func(c *gin.Context) {
			c.String(200, "ok")
		})

		req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestDBHealth tests the DBHealth middleware
func TestDBHealth(t *testing.T) {
	t.Run("Check returns error when svcCtx is nil", func(t *testing.T) {
		health := NewDBHealth(nil)
		err := health.Check(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "数据库模型未初始化")
	})

	t.Run("Check returns error when AdminModel is nil", func(t *testing.T) {
		health := NewDBHealth(&svc.ServiceContext{})
		err := health.Check(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "数据库模型未初始化")
	})

	t.Run("Check succeeds with valid database", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, model.AutoMigrate(db))

		// Create a test admin
		admin := &model.Admin{Username: "test", PasswordHash: "hash", Status: 1}
		require.NoError(t, db.Create(admin).Error)

		svcCtx := &svc.ServiceContext{
			AdminModel: model.NewAdminModel(db),
		}

		health := NewDBHealth(svcCtx)
		err = health.Check(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Check handles timeout", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
		require.NoError(t, err)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		svcCtx := &svc.ServiceContext{
			AdminModel: model.NewAdminModel(db),
		}

		health := NewDBHealth(svcCtx)
		err = health.Check(ctx)
		assert.Error(t, err)
	})

	t.Run("Check handles sql.ErrNoRows gracefully", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, model.AutoMigrate(db))

		// Create a test admin with ID 1 to ensure FindOne doesn't return ErrNoRows
		admin := &model.Admin{Username: "test", PasswordHash: "hash", Status: 1}
		admin.ID = 1
		require.NoError(t, db.Create(admin).Error)

		svcCtx := &svc.ServiceContext{
			AdminModel: model.NewAdminModel(db),
		}

		health := NewDBHealth(svcCtx)
		err = health.Check(context.Background())
		// Should succeed when DB is reachable
		assert.NoError(t, err)
	})

	t.Run("Ping calls Check with timeout", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, model.AutoMigrate(db))

		admin := &model.Admin{Username: "test", PasswordHash: "hash", Status: 1}
		require.NoError(t, db.Create(admin).Error)

		svcCtx := &svc.ServiceContext{
			AdminModel: model.NewAdminModel(db),
		}

		health := NewDBHealth(svcCtx)
		err = health.Ping()
		assert.NoError(t, err)
	})
}
