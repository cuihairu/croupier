package meta

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Proof of a dead branch: Service.Root always returns a nil error under every
// svcCtx construction, so the err != nil branch of Root in handler.go is
// unreachable and cannot be covered by new tests.
func TestService_Root_NeverReturnsError_Matrix(t *testing.T) {
	cases := []struct {
		name   string
		svcCtx *svc.ServiceContext
	}{
		{"empty context", &svc.ServiceContext{Config: config.Config{}}},
		{"dev mode nil profiles", &svc.ServiceContext{Config: config.Config{Server: config.ServerConfig{Mode: "dev"}}}},
		{"prod mode many profiles", &svc.ServiceContext{Config: config.Config{
			Server:   config.ServerConfig{Mode: "prod"},
			Profiles: map[string]config.ProfileConfig{"a": {}, "z": {}, "m": {}, "b": {}},
		}}},
		{"feature flags mixed", &svc.ServiceContext{Config: config.Config{
			FeatureFlags: config.FeatureFlagsConfig{
				config.FlagDev:       true,
				config.FlagSupport:   false,
				config.FlagAnalytics: true,
			},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := NewService(tc.svcCtx).Root(context.Background())
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "croupier-server", resp.Service)
			assert.NotEmpty(t, resp.Timestamp)
		})
	}
}

// enabledFeatures L2 gating semantics: a domain explicitly set to false at L2
// never appears; everything else composes fail-open.
func TestEnabledFeatures_Composition(t *testing.T) {
	allL2Off := config.FeatureFlagsConfig{
		config.FlagDev:        false,
		config.FlagSupport:    false,
		config.FlagAnalytics:  false,
		config.FlagOps:        false,
		config.FlagExtensions: false,
	}
	mixed := config.FeatureFlagsConfig{
		config.FlagDev:        true,
		config.FlagSupport:    false,
		config.FlagAnalytics:  true,
		config.FlagOps:        true,
		config.FlagExtensions: false,
	}
	layered := &settings.Layered{}

	assert.Equal(t, []string{"alerts", "functions", "registry"}, enabledFeatures(allL2Off, layered))
	assert.Equal(t,
		[]string{"alerts", "analytics", "dev", "extensions", "functions", "ops", "registry", "support"},
		enabledFeatures(config.FeatureFlagsConfig{}, layered))
	assert.Equal(t,
		[]string{"alerts", "analytics", "dev", "functions", "ops", "registry"},
		enabledFeatures(mixed, layered))
	// A nil Layered is fail-open just like a zero-value one
	assert.Equal(t,
		enabledFeatures(config.FeatureFlagsConfig{}, layered),
		enabledFeatures(config.FeatureFlagsConfig{}, nil))
}

// JSON contract of RootResponse: all seven fields must be serialized out.
func TestHandler_Root_ResponseContract(t *testing.T) {
	handler := newMetaHandler(map[string]config.ProfileConfig{"prod": {}}, "prod")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1", handler.Root)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for _, key := range []string{"service", "version", "environment", "timestamp", "features", "profiles", "links"} {
		assert.Contains(t, raw, key)
	}
}

// Concurrent requests to Root: shared state such as versionOnce and timestamps
// keeps yielding 200 under concurrency.
func TestHandler_Root_ConcurrentRequests(t *testing.T) {
	handler := newMetaHandler(map[string]config.ProfileConfig{"dev": {}, "prod": {}}, "dev")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1", handler.Root)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1", nil))
			assert.Equal(t, http.StatusOK, rec.Code)
		}()
	}
	wg.Wait()
}

// CROUPIER_VERSION set to pure whitespace is ignored, falling back to the
// VERSION file.
func TestCurrentAPIVersion_WhitespaceEnvFallsBackToFile(t *testing.T) {
	versionOnce = sync.Once{}
	apiVersion = ""
	t.Setenv("CROUPIER_VERSION", "   ")

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("9.9.9\n"), 0644))
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	assert.Equal(t, "9.9.9", currentAPIVersion())
}

// When the VERSION file contains only whitespace, readVersionFile returns an
// empty string (which then falls back to dev).
func TestReadVersionFile_WhitespaceContent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("  \n\t "), 0644))
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	assert.Equal(t, "", readVersionFile())
}

// once semantics of currentAPIVersion: resolved and cached on the first call;
// later environment changes do not affect the result.
func TestCurrentAPIVersion_CachedAfterFirstResolve(t *testing.T) {
	versionOnce = sync.Once{}
	apiVersion = ""
	t.Setenv("CROUPIER_VERSION", "cached-1.0")
	assert.Equal(t, "cached-1.0", currentAPIVersion())

	t.Setenv("CROUPIER_VERSION", "cached-2.0")
	assert.Equal(t, "cached-1.0", currentAPIVersion(), "version must be resolved exactly once")
}

// failingRootService injects an error through the RootProvider seam to cover the
// handler's error branch.
type failingRootService struct{}

func (failingRootService) Root(context.Context) (*RootResponse, error) {
	return nil, errors.New("meta service unavailable")
}

func TestHandler_Root_InjectedServiceError(t *testing.T) {
	handler := &Handler{service: failingRootService{}}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1", handler.Root)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "meta service unavailable")
}
