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

// 死分支实证：Service.Root 在各种 svcCtx 构造下恒返回 nil error，
// 因此 handler.go 中 Root 的 err != nil 分支不可达，无法通过新增测试覆盖。
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

// enabledFeatures 的 L2 门控语义：L2 显式 false 的域不出现，其余按 fail-open 合成。
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
	// nil Layered 与零值 Layered 同样 fail-open
	assert.Equal(t,
		enabledFeatures(config.FeatureFlagsConfig{}, layered),
		enabledFeatures(config.FeatureFlagsConfig{}, nil))
}

// RootResponse 的 JSON 契约：七个字段必须全部序列化输出。
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

// 并发请求 Root：versionOnce/时间戳等共享状态在并发下稳定输出 200。
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

// CROUPIER_VERSION 为纯空白时被忽略，回落到 VERSION 文件。
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

// VERSION 文件内容为纯空白时 readVersionFile 返回空串（进而回落 dev）。
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

// currentAPIVersion 的 once 语义：首次解析后缓存，后续环境变化不影响结果。
func TestCurrentAPIVersion_CachedAfterFirstResolve(t *testing.T) {
	versionOnce = sync.Once{}
	apiVersion = ""
	t.Setenv("CROUPIER_VERSION", "cached-1.0")
	assert.Equal(t, "cached-1.0", currentAPIVersion())

	t.Setenv("CROUPIER_VERSION", "cached-2.0")
	assert.Equal(t, "cached-1.0", currentAPIVersion(), "version must be resolved exactly once")
}

// failingRootService 通过 RootProvider 缝隙注入错误，覆盖 handler 错误分支。
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
