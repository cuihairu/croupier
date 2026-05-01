package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

func TestHandler_Healthz_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.GET("/healthz", handler.Healthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_Metrics_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.GET("/metrics", handler.Metrics)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_Status_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.GET("/status", handler.Status)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_Healthz_NilStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
	}
	service := NewService(svcCtx)

	resp, err := service.Healthz(context.Background(), &HealthzRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Without registry and DB, should not be OK
	assert.False(t, resp.OK)
	assert.NotEmpty(t, resp.Components.Database)
	assert.NotEmpty(t, resp.Components.Registry)
	assert.NotEmpty(t, resp.Components.Ops)
}

func TestService_Healthz_WithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: registry.NewStore(),
		Config: config.Config{
			Database: config.DatabaseConfig{
				Driver: "sqlite",
			},
		},
	}
	service := NewService(svcCtx)

	resp, err := service.Healthz(context.Background(), &HealthzRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// DB should be OK
	assert.True(t, resp.Components.Database["ok"].(bool))
	assert.Equal(t, "sqlite", resp.Components.Database["driver"])
}

func TestService_Metrics_WithAgents(t *testing.T) {
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "test-agent",
		GameID:    "game1",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
	}
	service := NewService(svcCtx)

	resp, err := service.Metrics(context.Background(), &MetricsRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Timestamp)
	assert.NotNil(t, resp.Counts)
	assert.Equal(t, 1, resp.Counts["agentsTotal"])
	assert.Equal(t, 1, resp.Counts["agentsHealthy"])
}

func TestService_Status_WithExpiredAgent(t *testing.T) {
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "expired-agent",
		GameID:    "game1",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(-1 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
	}
	service := NewService(svcCtx)

	resp, err := service.Status(context.Background(), &StatusRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.OK) // No DB, so not OK
	assert.Len(t, resp.Agents, 1)
}

func TestService_Status_NilContext(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	resp, err := service.Status(nil, &StatusRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestComponentHealthy_Nil(t *testing.T) {
	assert.False(t, componentHealthy(nil))
}

func TestComponentHealthy_NoOkField(t *testing.T) {
	status := map[string]interface{}{
		"error": "something",
	}
	assert.False(t, componentHealthy(status))
}

func TestComponentHealthy_OkTrue(t *testing.T) {
	status := map[string]interface{}{
		"ok": true,
	}
	assert.True(t, componentHealthy(status))
}

func TestComponentHealthy_OkFalse(t *testing.T) {
	status := map[string]interface{}{
		"ok": false,
	}
	assert.False(t, componentHealthy(status))
}

func TestComponentHealthy_OkNotBool(t *testing.T) {
	status := map[string]interface{}{
		"ok": "yes",
	}
	assert.False(t, componentHealthy(status))
}

func TestUptimeSeconds_Positive(t *testing.T) {
	uptime := uptimeSeconds()
	assert.GreaterOrEqual(t, uptime, int64(0))
}

func TestFirstNonNil_AllNil(t *testing.T) {
	assert.Nil(t, firstNonNil(nil, nil, nil))
}

func TestFirstNonNil_FirstNonNil(t *testing.T) {
	assert.Equal(t, "first", firstNonNil(nil, "first", "second"))
}

func TestFirstNonNil_MiddleNonNil(t *testing.T) {
	assert.Equal(t, "value", firstNonNil("value", nil, "other"))
}

func TestFirstNonNil_LastNonNil(t *testing.T) {
	assert.Equal(t, "last", firstNonNil(nil, nil, "last"))
}

func TestNormalizeAgentSnapshot_Nil(t *testing.T) {
	assert.Nil(t, normalizeAgentSnapshot(nil))
}

func TestNormalizeAgentSnapshot_Empty(t *testing.T) {
	result := normalizeAgentSnapshot(map[string]interface{}{})
	assert.NotNil(t, result)
	assert.Empty(t, result["id"])
}

func TestNormalizeAgentSnapshot_WithAllFields(t *testing.T) {
	item := map[string]interface{}{
		"id":              "agent-1",
		"agent_id":        "agent-1-alias",
		"game_id":         "game1",
		"gameId":          "game1-alt",
		"env":             "dev",
		"type":            "test",
		"addr":            "127.0.0.1:19091",
		"rpc_addr":        "127.0.0.1:19092",
		"ip":              "192.168.1.1",
		"version":         "1.0.0",
		"region":          "us-east",
		"zone":            "1a",
		"labels":          map[string]string{"key": "value"},
		"functions":       []string{"func1", "func2"},
		"providers":       []map[string]interface{}{{"provider_id": "p1"}},
		"providers_count": 2,
		"healthy":         true,
		"expires_in_sec":  300,
		"last_seen":       time.Now(),
		"active_conns":    10,
		"total_requests":  1000,
		"failed_requests": 10,
		"error_rate":      0.01,
		"avg_latency_ms":  50,
		"qps_limit":       100,
		"qps_1m":          80,
	}

	result := normalizeAgentSnapshot(item)
	assert.Equal(t, "agent-1", result["id"])
	assert.Equal(t, "agent-1-alias", result["agentId"]) // firstNonNil returns agent_id first
	assert.Equal(t, "game1", result["gameId"])          // firstNonNil returns game_id first
	assert.Equal(t, "dev", result["env"])
	assert.Equal(t, "127.0.0.1:19091", result["addr"])
	assert.Equal(t, "127.0.0.1:19092", result["rpcAddr"]) // firstNonNil returns rpc_addr first
	assert.NotNil(t, result["providers"])
}

func TestNormalizeAgentSnapshots_Empty(t *testing.T) {
	result := normalizeAgentSnapshots([]map[string]interface{}{})
	assert.Empty(t, result)
}

func TestNormalizeAgentSnapshots_Multiple(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "agent-1", "addr": "127.0.0.1:19091"},
		{"id": "agent-2", "addr": "127.0.0.1:19092"},
	}
	result := normalizeAgentSnapshots(items)
	assert.Len(t, result, 2)
}

func TestNormalizeProviders_Nil(t *testing.T) {
	assert.Nil(t, normalizeProviders(nil))
}

func TestNormalizeProviders_NotSlice(t *testing.T) {
	assert.Nil(t, normalizeProviders("not a slice"))
}

func TestNormalizeProviders_EmptySlice(t *testing.T) {
	assert.Nil(t, normalizeProviders([]map[string]interface{}{}))
}

func TestNormalizeProviders_WithItems(t *testing.T) {
	items := []map[string]interface{}{
		{
			"provider_id":    "p1",
			"game_id":        "game1",
			"addr":           "127.0.0.1:19091",
			"version":        "1.0.0",
			"last_seen_unix": int64(1234567890),
			"function_ids":   []string{"f1", "f2"},
			"functions":      []string{"func1", "func2"},
		},
	}
	result := normalizeProviders(items)
	assert.Len(t, result, 1)
	assert.Equal(t, "p1", result[0]["providerId"])
	assert.Equal(t, "game1", result[0]["gameId"])
	assert.Equal(t, "127.0.0.1:19091", result[0]["addr"])
}

func TestCollectRegistryStats_NilStore(t *testing.T) {
	stats, snapshots := collectRegistryStats(nil)
	assert.False(t, stats["ok"].(bool))
	assert.Contains(t, stats["error"], "not initialized")
	assert.Empty(t, snapshots)
}

func TestCheckDatabaseHealth_NilContext(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	result := checkDatabaseHealth(nil, svcCtx)
	assert.False(t, result["ok"].(bool))
	assert.Contains(t, result["error"], "not initialized")
}

func TestSummarizeOpsState_NilContext(t *testing.T) {
	result := summarizeOpsState(nil)
	assert.False(t, result["ok"].(bool))
	assert.Contains(t, result["error"], "not initialized")
}

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)
	assert.NotNil(t, service)
	assert.Equal(t, svcCtx, service.svcCtx)
}

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)
	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestService_Healthz_WithOpsState(t *testing.T) {
	opsStore := svc.NewOpsStateStore("")
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		OpsStateStore: opsStore,
	}
	service := NewService(svcCtx)

	resp, err := service.Healthz(context.Background(), &HealthzRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// With OpsState but no DB, still not fully OK
	assert.False(t, resp.OK)
	assert.NotNil(t, resp.Components.Ops)
}

func TestService_Metrics_WithOpsState(t *testing.T) {
	opsStore := svc.NewOpsStateStore("")
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		OpsStateStore: opsStore,
	}
	service := NewService(svcCtx)

	resp, err := service.Metrics(context.Background(), &MetricsRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Ops)
}

func TestService_Status_FullyConfigured(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	opsStore := svc.NewOpsStateStore("")
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "test-agent",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
	})

	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: store,
		OpsStateStore: opsStore,
		Config: config.Config{
			Database: config.DatabaseConfig{
				Driver: "sqlite",
			},
		},
	}
	service := NewService(svcCtx)

	resp, err := service.Status(context.Background(), &StatusRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.OK)
	assert.Len(t, resp.Agents, 1)
}
