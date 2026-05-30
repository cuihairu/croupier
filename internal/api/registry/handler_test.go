package registry

import (
	"bytes"
	"encoding/json"
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

func TestHandler_GetRegistry_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create service with minimal context
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	// Register a test agent
	store := svcCtx.RegistryStore
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "test-agent-1",
		GameID:    "game1",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	router := gin.New()
	router.POST("/registry", func(c *gin.Context) {
		c.Set("username", "admin")
		handler.GetRegistry(c)
	})

	reqBody := `{}`
	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp RegistryResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Agents)
}

func TestHandler_GetRegistry_WithInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	reqBody := `{invalid json`
	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should still succeed with empty request
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_GetRegistry_NilStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
	}
	service := NewService(svcCtx)

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Agents)
	assert.Empty(t, resp.Functions)
}

func TestService_GetRegistry_EmptyStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Agents)
	assert.Empty(t, resp.Functions)
	assert.Empty(t, resp.Coverage)
}

func TestService_GetRegistry_MultipleAgents(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add multiple agents in non-sorted order
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-z",
		GameID:   "game2",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"func.a": {Enabled: true},
		},
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-a",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"func.b": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 2)
	// Should be sorted by gameID then agentID
	assert.Equal(t, "agent-a", resp.Agents[0].AgentID)
	assert.Equal(t, "agent-z", resp.Agents[1].AgentID)
}

func TestService_GetRegistry_ExpiredAgent(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add expired agent
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "expired-agent",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(-1 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"test.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	// Agent should be present but unhealthy
	assert.Equal(t, false, resp.Agents[0].Healthy)
	assert.Equal(t, 0, resp.Agents[0].ExpiresInSec)
}

func TestService_GetRegistry_AgentWithNilExpireAt(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with zero ExpireAt
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "no-expiry-agent",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Time{},
		Functions: map[string]registry.FunctionMeta{
			"test.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	assert.Equal(t, false, resp.Agents[0].Healthy)
	assert.Equal(t, 0, resp.Agents[0].ExpiresInSec)
}

func TestService_GetRegistry_AgentWithZeroID(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with empty ID (should be skipped)
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"test.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 0)
}

func TestService_GetRegistry_MultipleAgentsSameFunction(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Two agents with the same function
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"shared.func": {Enabled: true},
		},
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-2",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"shared.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 2)
	assert.Len(t, resp.Functions, 1)
	// Function should have both agents
	assert.Len(t, resp.Functions[0].Agents, 2)
}

func TestService_GetRegistry_DisabledFunctionNotIncluded(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Agent with disabled function
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "dev",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"disabled.func": {Enabled: false},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Agents, 1)
	assert.Len(t, resp.Functions, 0)
}

func TestTtlAndHealth_NilSession(t *testing.T) {
	ttl, healthy := ttlAndHealth(nil)
	assert.Equal(t, 0, ttl)
	assert.Equal(t, false, healthy)
}

func TestBuildGameEnvKey(t *testing.T) {
	tests := []struct {
		name     string
		gameID   string
		env      string
		expected string
	}{
		{"normal", "game1", "dev", "game1|dev"},
		{"with spaces", " game1 ", " dev ", "game1|dev"},
		{"empty env", "game1", "", "game1|"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildGameEnvKey(tt.gameID, tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}
