package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogSearchRouter(t *testing.T, scopeGame, scopeEnv string) (*gin.Engine, *svc.ServiceContext) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentLogEntryRecord{}))
	// Need registry + session for scope check. Use minimal: register an agent
	// session in memory? requireAgentScope consults RegistryStore.
	// Simpler: build ServiceContext with a registry Store and seed a session.
	rstore := reg.NewStore()
	svcCtx := &svc.ServiceContext{
		RegistryStore: rstore,
		LogStore:      reg.NewLogStore(),
		DB:            db,
	}
	svcCtx.LogStore.SetDB(db)
	// seed agent session
	require.NoError(t, rstore.UpsertAgent(&reg.AgentSession{
		AgentID: "sidecar-1",
		GameID:  scopeGame,
		Env:     scopeEnv,
	}))

	// scope middleware
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: scopeGame, Env: scopeEnv})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	h := NewHandler(NewService(svcCtx))
	r.GET("/ops/logs/search", h.AgentLogSearch)
	return r, svcCtx
}

func TestAgentLogSearch_ReturnsFilteredRows(t *testing.T) {
	r, svcCtx := setupLogSearchRouter(t, "game-a", "prod")
	svcCtx.LogStore.Ingest("sidecar-1", []*opsv1.LogEntry{
		{File: "/var/log/a.log", Level: "info", Message: "first", Seq: 1},
		{File: "/var/log/a.log", Level: "error", Message: "boom", Seq: 2},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ops/logs/search?agentId=sidecar-1", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body AgentLogSearchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "sidecar-1", body.AgentID)
	assert.Len(t, body.Items, 2)
	assert.Equal(t, "boom", body.Items[0].Message)
	assert.Equal(t, "error", body.Items[0].Level)
}

func TestAgentLogSearch_LevelFilter(t *testing.T) {
	r, svcCtx := setupLogSearchRouter(t, "game-a", "prod")
	svcCtx.LogStore.Ingest("sidecar-1", []*opsv1.LogEntry{
		{File: "f", Level: "info", Message: "i", Seq: 1},
		{File: "f", Level: "warn", Message: "w", Seq: 2},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ops/logs/search?agentId=sidecar-1&level=warn", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body AgentLogSearchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Items, 1)
	assert.Equal(t, "w", body.Items[0].Message)
}
