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
	ctx.Request = httptest.NewRequest("POST", "/agent/filters", bytes.NewBuffer([]byte(`{"game_id": "test"}`)))
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
	ctx.Request = httptest.NewRequest("POST", "/agent/meta", bytes.NewBuffer([]byte(`{"agent_id": "test"}`)))
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
