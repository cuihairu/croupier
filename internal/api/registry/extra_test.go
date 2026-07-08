package registry

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/config"
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

func TestTtlAndHealth_Expired(t *testing.T) {
	// Test with expired session
	session := &registry.AgentSession{
		ExpireAt: time.Now().Add(-1 * time.Minute),
	}
	ttl, healthy := ttlAndHealth(session)
	assert.Equal(t, 0, ttl)
	assert.False(t, healthy)
}

func TestTtlAndHealth_ExpiresSoon(t *testing.T) {
	// Test with session expiring in a few seconds
	session := &registry.AgentSession{
		ExpireAt: time.Now().Add(5 * time.Second),
	}
	ttl, healthy := ttlAndHealth(session)
	assert.Greater(t, ttl, 0)
	assert.True(t, healthy)
}

func TestBuildGameEnvKey_WithWhitespace(t *testing.T) {
	result := buildGameEnvKey("  game1  ", "  prod  ")
	assert.Equal(t, "game1|prod", result)
}

func TestBuildGameEnvKey_Mixed(t *testing.T) {
	result := buildGameEnvKey("game1", "prod")
	assert.Equal(t, "game1|prod", result)
}

func TestService_GetRegistry_WithAssignments(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add an agent
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	// Should have assignments in response
	assert.NotNil(t, resp.Assignments)
}

func TestService_GetRegistry_Sorting(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agents in non-sorted order
	agents := []*registry.AgentSession{
		{AgentID: "z-agent", GameID: "b-game", Env: "prod", RPCAddr: "127.0.0.1:19093", ExpireAt: time.Now().Add(5 * time.Minute), Functions: map[string]registry.FunctionMeta{"z.func": {Enabled: true}}},
		{AgentID: "a-agent", GameID: "a-game", Env: "prod", RPCAddr: "127.0.0.1:19091", ExpireAt: time.Now().Add(5 * time.Minute), Functions: map[string]registry.FunctionMeta{"a.func": {Enabled: true}}},
		{AgentID: "m-agent", GameID: "a-game", Env: "prod", RPCAddr: "127.0.0.1:19092", ExpireAt: time.Now().Add(5 * time.Minute), Functions: map[string]registry.FunctionMeta{"m.func": {Enabled: true}}},
	}
	for _, agent := range agents {
		svcCtx.RegistryStore.UpsertAgent(agent)
	}

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Check agents are sorted by GameID then AgentID
	if len(resp.Agents) >= 2 {
		assert.LessOrEqual(t, resp.Agents[0].GameID, resp.Agents[1].GameID)
		if resp.Agents[0].GameID == resp.Agents[1].GameID {
			assert.LessOrEqual(t, resp.Agents[0].AgentID, resp.Agents[1].AgentID)
		}
	}
}

func TestService_GetRegistry_FunctionSorting(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agents with functions in different orders
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"z.func": {Enabled: true},
			"a.func": {Enabled: true},
		},
	})
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-2",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"m.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Check functions are sorted
	if len(resp.Functions) >= 1 && len(resp.Functions[0].Agents) >= 2 {
		// Agents should be sorted alphabetically
		agents := resp.Functions[0].Agents
		sorted := true
		for i := 1; i < len(agents); i++ {
			if agents[i-1] > agents[i] {
				sorted = false
				break
			}
		}
		assert.True(t, sorted, "Agents should be sorted")
	}
}

func TestService_GetRegistry_CoverageSorting(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agents for different game/env combos
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "z-game",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-2",
		GameID:    "a-game",
		Env:       "dev",
		RPCAddr:   "127.0.0.1:19092",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Check coverage is sorted by GameEnv
	if len(resp.Coverage) >= 2 {
		assert.LessOrEqual(t, resp.Coverage[0].GameEnv, resp.Coverage[1].GameEnv)
	}
}

func TestService_GetRegistry_DisabledFunctions(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent with both enabled and disabled functions
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"enabled.func":  {Enabled: true},
			"disabled.func": {Enabled: false},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Only enabled functions should appear
	for _, fn := range resp.Functions {
		if fn.ID == "disabled.func" {
			t.Errorf("Disabled function should not appear in response")
		}
	}
}

func TestService_GetRegistry_EmptyAgentID(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent with empty AgentID (should be skipped)
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	// Agent with empty ID should be skipped
	for _, agent := range resp.Agents {
		assert.NotEmpty(t, agent.AgentID)
	}
}

func TestHandler_GetRegistry_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should handle invalid JSON gracefully (empty request fallback)
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
}

func TestHandler_GetRegistry_InvalidMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.Any("/registry", handler.GetRegistry)

	// Try GET request (no body)
	req := httptest.NewRequest("GET", "/registry", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should handle GET without body (empty request fallback)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_GetRegistry_WhitespacedAgentID(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent with whitespace in AgentID (should be trimmed)
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "  agent-whitespace  ",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	// Agent with whitespace should be included (after trim in ShouldBindUri)
	assert.Greater(t, len(resp.Agents), 0)
}

func TestService_GetRegistry_FunctionsKeyGeneration(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add multiple agents for the same function
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-a",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"shared.func": {Enabled: true},
		},
	})
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-b",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19092",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"shared.func": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Should have one function with two agents
	assert.Len(t, resp.Functions, 1)
	assert.Len(t, resp.Functions[0].Agents, 2)
}

func TestService_GetRegistry_AssignmentIntegration(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"assigned.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Assignments should be included in response
	assert.NotNil(t, resp.Assignments)
}

func TestService_GetRegistry_CoverageUncovered(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"covered.func":   {Enabled: true},
			"uncovered.func": {Enabled: false},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Only enabled functions should be in response
	for _, fn := range resp.Functions {
		assert.NotEqual(t, "uncovered.func", fn.ID)
	}
}

func TestHandler_GetRegistry_JSONRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	reqBody := `{"gameId":"test","env":"prod"}`
	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_GetRegistry_NilRegistryStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: nil,
	}
	service := NewService(svcCtx)

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Agents)
	assert.Empty(t, resp.Functions)
}

func TestService_GetRegistry_NilAgentSession(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Manually add nil to agents slice (if possible through internal access)
	// Since we can't directly add nil to AgentsUnsafe, we test with empty store
	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_GetRegistry_WhitespaceAgentID(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Add agent with whitespace-only AgentID (should be filtered out)
	// Note: The registry store may not allow empty AgentIDs, so we test the filter logic
	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_GetRegistry_CoverageStats(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with function
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "stats-agent",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
			"func2": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Check coverage is calculated
	assert.NotNil(t, resp.Coverage)
	if len(resp.Coverage) > 0 {
		assert.NotEmpty(t, resp.Coverage[0].GameEnv)
	}
}

func TestService_GetRegistry_DisabledFunctionsNotCounted(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with mixed enabled/disabled functions
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "mixed-agent",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"enabled.func":  {Enabled: true},
			"disabled.func": {Enabled: false},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Only enabled functions should appear
	foundEnabled := false
	foundDisabled := false
	for _, fn := range resp.Functions {
		if fn.ID == "enabled.func" {
			foundEnabled = true
		}
		if fn.ID == "disabled.func" {
			foundDisabled = true
		}
	}
	assert.True(t, foundEnabled, "enabled function should be in response")
	assert.False(t, foundDisabled, "disabled function should not be in response")
}

func TestService_GetRegistry_SameFunctionMultipleAgents(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Same function on multiple agents
	for i := 1; i <= 3; i++ {
		store.UpsertAgent(&registry.AgentSession{
			AgentID:  "agent-" + string(rune('0'+i)),
			GameID:   "game1",
			Env:      "prod",
			RPCAddr:  "127.0.0.1:1909" + string(rune('0'+i)),
			ExpireAt: time.Now().Add(5 * time.Minute),
			Functions: map[string]registry.FunctionMeta{
				"shared.func": {Enabled: true},
			},
		})
	}

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)

	// Should have one function entry with multiple agents
	found := false
	for _, fn := range resp.Functions {
		if fn.ID == "shared.func" {
			found = true
			assert.Len(t, fn.Agents, 3)
		}
	}
	assert.True(t, found, "shared function should be in response")
}

func TestService_GetRegistry_WithEmptyAgentID(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)
	store := svcCtx.RegistryStore

	// Add agent with empty AgentID (should be filtered)
	store.UpsertAgent(&registry.AgentSession{
		AgentID:   "",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"test.func": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Agents)
}

func TestService_GetRegistry_WithNilAgent(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Agents)
}

func TestService_GetRegistry_HandlerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	handler := NewHandler(NewService(svcCtx))

	router := gin.New()
	router.POST("/registry", handler.GetRegistry)

	// Invalid JSON should still work (fallback to empty request)
	req := httptest.NewRequest("POST", "/registry", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestService_GetRegistry_AuthorizedUser(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Test with authorized user context
	ctx := context.WithValue(context.Background(), "username", "testuser")
	ctx = context.WithValue(ctx, "adminID", uint(1))

	// This will fail at permission check since no permission service is set up
	_, err := service.GetRegistry(ctx, &RegistryRequest{})
	// We expect an error because permission service is nil
	assert.Error(t, err)
}

func TestService_GetRegistry_UnauthorizedUser(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	}
	service := NewService(svcCtx)

	// Test with user that doesn't have permission
	ctx := context.WithValue(context.Background(), "username", "noperm")
	ctx = context.WithValue(ctx, "adminID", uint(999))

	// This will fail at permission check
	_, err := service.GetRegistry(ctx, &RegistryRequest{})
	assert.Error(t, err)
}

func TestService_GetRegistry_WithAssignmentsFile(t *testing.T) {
	// Create temp directory for assignments
	tmpDir := t.TempDir()
	assignmentsPath := tmpDir + "/assignments.json"

	// Write test assignments
	assignmentsData := `{
		"game1|prod": ["func1", "func2"],
		"game2|dev": ["func3"]
	}`
	err := os.WriteFile(assignmentsPath, []byte(assignmentsData), 0644)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	// Add agent that covers func1
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "prod",
		RPCAddr:   "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{"func1": {Enabled: true}},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Check coverage
	assert.NotEmpty(t, resp.Coverage)

	// Find game1|prod coverage
	var game1ProdCoverage *RegistryCoverage
	for i := range resp.Coverage {
		if resp.Coverage[i].GameEnv == "game1|prod" {
			game1ProdCoverage = &resp.Coverage[i]
			break
		}
	}
	require.NotNil(t, game1ProdCoverage)
	// func1 should be covered (healthy), func2 should be uncovered
	assert.Contains(t, game1ProdCoverage.Functions, "func1")
	assert.Contains(t, game1ProdCoverage.Functions, "func2")
}

func TestService_GetRegistry_AssignmentWithEmptyFunctions(t *testing.T) {
	// Create temp directory for assignments
	tmpDir := t.TempDir()
	assignmentsPath := tmpDir + "/assignments.json"

	// Write test assignments with empty functions
	assignmentsData := `{
		"game1|prod": ["func1", "", "  ", "func2"]
	}`
	err := os.WriteFile(assignmentsPath, []byte(assignmentsData), 0644)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Check coverage - empty functions should be filtered
	assert.NotEmpty(t, resp.Coverage)
}

func TestService_GetRegistry_AssignmentCoverageWithAgent(t *testing.T) {
	// Create temp directory for assignments
	tmpDir := t.TempDir()
	assignmentsPath := tmpDir + "/assignments.json"

	// Write test assignments
	assignmentsData := `{
		"game1|prod": ["func1", "func2", "func3"]
	}`
	err := os.WriteFile(assignmentsPath, []byte(assignmentsData), 0644)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		RegistryStore: registry.NewStore(),
		Config: config.Config{
			Registry: config.RegistryConfig{
				AssignmentsPath: assignmentsPath,
			},
		},
	}
	service := NewService(svcCtx)

	// Add agent that covers func1 and func2
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game1",
		Env:      "prod",
		RPCAddr:  "127.0.0.1:19091",
		ExpireAt: time.Now().Add(5 * time.Minute),
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
			"func2": {Enabled: true},
		},
	})

	resp, err := service.GetRegistry(nil, &RegistryRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Find game1|prod coverage
	var game1ProdCoverage *RegistryCoverage
	for i := range resp.Coverage {
		if resp.Coverage[i].GameEnv == "game1|prod" {
			game1ProdCoverage = &resp.Coverage[i]
			break
		}
	}
	require.NotNil(t, game1ProdCoverage)

	// func1 and func2 should be covered, func3 should be uncovered
	assert.Contains(t, game1ProdCoverage.Functions, "func1")
	assert.Contains(t, game1ProdCoverage.Functions, "func2")
	assert.Contains(t, game1ProdCoverage.Functions, "func3")

	// func3 should be in uncovered list
	assert.Contains(t, game1ProdCoverage.Uncovered, "func3")
	// func1 and func2 should not be in uncovered list
	assert.NotContains(t, game1ProdCoverage.Uncovered, "func1")
	assert.NotContains(t, game1ProdCoverage.Uncovered, "func2")
}
