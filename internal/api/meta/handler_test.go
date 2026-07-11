package meta

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMetaHandler(profiles map[string]config.ProfileConfig, mode string) *Handler {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server:   config.ServerConfig{Mode: mode},
			Profiles: profiles,
		},
	}
	return NewHandler(NewService(svcCtx))
}

func newMetaContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1", nil)
	return ctx, rec
}

func TestHandler_Root_Success(t *testing.T) {
	handler := newMetaHandler(map[string]config.ProfileConfig{
		"prod": {},
		"dev":  {},
	}, "dev")

	ctx, rec := newMetaContext()
	handler.Root(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp RootResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "croupier-server", resp.Service)
	assert.NotEmpty(t, resp.Version)
	assert.Equal(t, "dev", resp.Environment)
	assert.NotEmpty(t, resp.Timestamp)
	assert.NotEmpty(t, resp.Features)
	// Profiles come back sorted.
	assert.Equal(t, []string{"dev", "prod"}, resp.Profiles)
	assert.Contains(t, resp.Links, "docs")
	assert.Contains(t, resp.Links, "status")
	assert.Contains(t, resp.Links, "health")
}

func TestHandler_Root_NoProfiles(t *testing.T) {
	handler := newMetaHandler(nil, "prod")

	ctx, rec := newMetaContext()
	handler.Root(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp RootResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Profiles)
	assert.Equal(t, "prod", resp.Environment)
}

func TestService_Root_Deterministic(t *testing.T) {
	svc := NewService(&svc.ServiceContext{
		Config: config.Config{
			Server:   config.ServerConfig{Mode: "test"},
			Profiles: map[string]config.ProfileConfig{"a": {}, "b": {}, "c": {}},
		},
	})

	resp, err := svc.Root(nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"a", "b", "c"}, resp.Profiles)
}
