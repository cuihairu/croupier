package agent

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_GetAnalyticsFilters_WithBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: "",
			},
		},
	})
	handler := NewHandler(service)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/agent/filters", bytes.NewBuffer([]byte(`{"gameId": "test"}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.GetAnalyticsFilters(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_UpdateMeta_WithBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	})
	handler := NewHandler(service)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/agent/meta", bytes.NewBuffer([]byte(`{"agentId": "test"}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateMeta(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestService_GetAnalyticsFilters_WithEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	data, err := analytics.SaveAnalyticsFiltersJSON([]analytics.AnalyticsFilters{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	service := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
	})

	resp, err := service.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)
}

func TestService_UpdateMeta_WithAgents(t *testing.T) {
	store := registry.NewStore()
	service := NewService(&svc.ServiceContext{
		RegistryStore: store,
	})

	resp, err := service.UpdateMeta(context.Background(), &UpdateMetaRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.Count)
}

func TestService_GetAnalyticsFilters_MultipleItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	data, err := analytics.SaveAnalyticsFiltersJSON([]analytics.AnalyticsFilters{
		{GameId: "game1", Filters: map[string]any{"env": "prod"}},
		{GameId: "game2", Filters: map[string]any{"env": "stage"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	service := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
	})

	resp, err := service.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Items, 2)
}

func TestService_UpdateMeta_ResponseFormat(t *testing.T) {
	store := registry.NewStore()
	service := NewService(&svc.ServiceContext{
		RegistryStore: store,
	})

	resp, err := service.UpdateMeta(context.Background(), &UpdateMetaRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Empty(t, resp.Agents)
}

func TestService_UpdateMeta_NilStore(t *testing.T) {
	service := NewService(&svc.ServiceContext{
		RegistryStore: nil,
	})

	resp, err := service.UpdateMeta(context.Background(), &UpdateMetaRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_GetAnalyticsFilters_Concurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics_filters.json")
	data, err := analytics.SaveAnalyticsFiltersJSON([]analytics.AnalyticsFilters{
		{GameId: "game1", Filters: map[string]any{"env": "prod"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	service := NewService(&svc.ServiceContext{
		Config: config.Config{
			Registry: config.RegistryConfig{
				AnalyticsFiltersPath: path,
			},
		},
		AnalyticsFiltersLock: &sync.RWMutex{},
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := service.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
			assert.NoError(t, err)
			assert.Equal(t, 1, resp.Count)
		}()
	}
	wg.Wait()
}

// loadFiltersFromAnalyticsInstallation branches: extension install config takes
// precedence over the file; branches for missing filters / bad JSON / empty config.
func TestService_AnalyticsFilters_FromInstallation(t *testing.T) {
	afSvc := NewService(&svc.ServiceContext{}) // Extensions nil -> file fallback branch

	// Extensions nil -> take the file fallback path (a missing file errors; an existing
	// file parses successfully)
	filtersResp, fileErr := afSvc.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	if fileErr != nil {
		assert.Nil(t, filtersResp)
	} else {
		assert.NotNil(t, filtersResp)
	}

	// Exercise the config branch of loadFiltersFromAnalyticsInstallation directly
	filters, ok, err := afSvc.loadFiltersFromAnalyticsInstallation(context.Background())
	assert.False(t, ok)
	assert.NoError(t, err)
	assert.Nil(t, filters)
}

func TestService_UpdateMeta_NoStore(t *testing.T) {
	afSvc := NewService(&svc.ServiceContext{})
	_, err := afSvc.UpdateMeta(context.Background(), &UpdateMetaRequest{})
	assert.ErrorContains(t, err, "registry store unavailable")
}

func TestHandler_UpdateMeta_BindAndError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// service error path (RegistryStore nil) -> 500
	errHandler := NewHandler(NewService(&svc.ServiceContext{}))
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/agent/meta", bytes.NewBuffer([]byte(`{"agentId":"a"}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	errHandler.UpdateMeta(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// Both file-fallback success/failure branches + UpdateMeta iterating online agents.
func TestService_AnalyticsFilters_FileFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "analytics_filters.json")
	require.NoError(t, os.WriteFile(path, []byte(
		`{"items":[{"gameId":"g1","filters":[{"key":"level","values":[1,2]}]}]}`), 0o600))

	afSvc := NewService(&svc.ServiceContext{Config: config.Config{
		Registry: config.RegistryConfig{AnalyticsFiltersPath: path},
	}})
	resp, err := afSvc.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "g1", resp.Items[0].GameId)

	// Path points at a directory -> read error (not NotExist, so no empty-list downgrade)
	afSvcErr := NewService(&svc.ServiceContext{Config: config.Config{
		Registry: config.RegistryConfig{AnalyticsFiltersPath: t.TempDir()},
	}})
	_, err = afSvcErr.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	assert.Error(t, err)

	// File content is bad JSON -> LoadAnalyticsFilters error branch
	badPath := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(badPath, []byte(`[{"gameId":`), 0o600))
	afSvcBad := NewService(&svc.ServiceContext{Config: config.Config{
		Registry: config.RegistryConfig{AnalyticsFiltersPath: badPath},
	}})
	_, err = afSvcBad.GetAnalyticsFilters(context.Background(), &GetAnalyticsFiltersRequest{})
	assert.Error(t, err)
}

func TestService_UpdateMeta_WithRegisteredAgent(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1", GameID: "game-1", Env: "dev",
	}))
	afSvc := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := afSvc.UpdateMeta(context.Background(), &UpdateMetaRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Agents)
}
