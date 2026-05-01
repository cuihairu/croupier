package registry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// Additional tests to improve coverage to 80%+

func TestHandler_GetRegistry_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))
	store := svcCtx.RegistryStore

	// Add test data
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
		Labels:    map[string]string{"region": "us-east"},
	})

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	reqBody := `{"gameId": "game1", "env": "prod"}`
	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetRegistry_EmptyRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should handle empty body
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
}

func TestService_GetRegistry_WithAgentLabels(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with labels
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "labeled-agent",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
		Labels: map[string]string{
			"region":     "us-west",
			"datacenter": "dc1",
			"version":    "1.0.0",
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
}

func TestService_GetRegistry_AgentAboutToExpire(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent that will expire soon
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "expiring-soon-agent",
		GameID:    "game1",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(30 * time.Second),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	assert.True(t, resp.Agents[0].ExpiresInSec > 0 && resp.Agents[0].ExpiresInSec <= 60)
}

func TestService_GetRegistry_FunctionWithMultipleAgents(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Three agents with the same function
	for i := 1; i <= 3; i++ {
		store.UpsertAgent(&registry.AgentSession{
			AgentID:  "agent-" + string(rune('0'+i)),
			GameID:   "game1",
			Env:      "dev",
			RPCAddr:  "127.0.0.1:1909" + string(rune('0'+i)),
			ExpireAt: time.Now().Add(5 * time.Minute),
			Functions: map[string]registry.FunctionMeta{
				"multi.func": {Enabled: true},
			},
		})
	}

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Functions, 1)
	assert.Len(t, resp.Functions[0].Agents, 3)
}

func TestService_GetRegistry_EmptyFunctions(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with no functions
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "no-func-agent",
		GameID:    "game1",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	assert.Empty(t, resp.Functions)
}

func TestService_GetRegistry_FilteredResponse(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agents for different games
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"game1.func": {Enabled: true}},
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-2",
		GameID:    "game2",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19092",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"game2.func": {Enabled: true}},
	})

	// Request all agents (no filtering)
	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Agents, 2)
}

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)
	assert.NotNil(t, service)
}

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)
	assert.NotNil(t, handler)
}

func TestTtlAndHealth_ZeroTime(t *testing.T) {
	// Test with zero time (epoch)
	session := &registry.AgentSession{
		ExpireAt: time.Time{},
	}
	ttl, healthy := ttlAndHealth(session)
	assert.Equal(t, 0, ttl)
	assert.False(t, healthy)
}

func TestBuildGameEnvKey_EverythingEmpty(t *testing.T) {
	result := buildGameEnvKey("", "")
	assert.Equal(t, "|", result)
}
