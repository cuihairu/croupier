package nng

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestServerWithDB tests the NNG server with database persistence
func TestServerWithDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&registry.AgentSessionDB{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create server with database
	addrs := []ListenAddr{ParseListenAddr(":0")} // Use random port
	store := registry.NewStoreWithDB(db)
	agentModel := registry.NewAgentSessionModel(db)
	server := NewServerWithDB(addrs, store, agentModel)

	if server == nil {
		t.Fatal("failed to create server")
	}

	if server.agentSessionLoader == nil {
		t.Fatal("agentSessionLoader should not be nil")
	}

	// Test LoadAgentSessions (should be empty initially)
	if err := server.LoadAgentSessions(); err != nil {
		t.Errorf("LoadAgentSessions failed: %v", err)
	}

	// Verify no sessions loaded
	server.Store().Mu().RLock()
	agents := server.Store().AgentsUnsafe()
	server.Store().Mu().RUnlock()

	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

// TestHandleRegisterWithDB tests agent registration with database persistence
func TestHandleRegisterWithDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&registry.AgentSessionDB{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create server with database
	addrs := []ListenAddr{ParseListenAddr(":0")}
	store := registry.NewStoreWithDB(db)
	agentModel := registry.NewAgentSessionModel(db)
	server := NewServerWithDB(addrs, store, agentModel)
	server.SetDefaultSessionTTL(5 * time.Minute)

	ctx := context.Background()

	// Create register request
	req := &agentv1.RegisterRequest{
		AgentId:    "test-agent-1",
		GameId:     "test-game",
		Env:        "test",
		RpcAddr:    "localhost:19090",
		Version:    "1.0.0",
		TtlSeconds: 300,
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:      "func1",
				Enabled: true,
				Version: "1.0",
			},
		},
		Processes: []*agentv1.AgentProcess{
			{
				ServiceId:    "provider1",
				Addr:         "localhost:19091",
				Version:      "1.0.0",
				LastSeenUnix: time.Now().Unix(),
				FunctionIds:  []string{"func1"},
			},
		},
	}

	// Handle registration
	resp, err := server.handleRegisterRequest(ctx, req)
	if err != nil {
		t.Fatalf("handleRegisterRequest failed: %v", err)
	}

	if resp == nil {
		t.Fatal("response should not be nil")
	}

	// Verify session in memory
	server.Store().Mu().RLock()
	agent := server.Store().AgentsUnsafe()["test-agent-1"]
	server.Store().Mu().RUnlock()

	if agent == nil {
		t.Fatal("agent not found in registry")
	}

	if agent.AgentID != "test-agent-1" {
		t.Errorf("expected agent_id test-agent-1, got %s", agent.AgentID)
	}

	if len(agent.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(agent.Providers))
	}

	// Verify session in database
	var dbSessions []registry.AgentSessionDB
	if err := db.Where("agent_id = ?", "test-agent-1").Find(&dbSessions).Error; err != nil {
		t.Fatalf("failed to query database: %v", err)
	}

	if len(dbSessions) != 1 {
		t.Fatalf("expected 1 session in database, got %d", len(dbSessions))
	}

	if dbSessions[0].AgentID != "test-agent-1" {
		t.Errorf("expected agent_id test-agent-1, got %s", dbSessions[0].AgentID)
	}

	if dbSessions[0].GameID != "test-game" {
		t.Errorf("expected game_id test-game, got %s", dbSessions[0].GameID)
	}
}

// TestHandleHeartbeatWithDB tests heartbeat with database update
func TestHandleHeartbeatWithDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&registry.AgentSessionDB{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create server with database
	addrs := []ListenAddr{ParseListenAddr(":0")}
	store := registry.NewStoreWithDB(db)
	agentModel := registry.NewAgentSessionModel(db)
	server := NewServerWithDB(addrs, store, agentModel)
	server.SetDefaultSessionTTL(5 * time.Minute)

	ctx := context.Background()

	// Register an agent first
	regReq := &agentv1.RegisterRequest{
		AgentId:    "test-agent-2",
		GameId:     "test-game",
		Env:        "test",
		RpcAddr:    "localhost:19090",
		Version:    "1.0.0",
		TtlSeconds: 300,
	}

	if _, err := server.handleRegisterRequest(ctx, regReq); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Wait a bit to ensure async database write completes
	time.Sleep(100 * time.Millisecond)

	// Get original expire_at
	server.Store().Mu().RLock()
	originalAgent := server.Store().AgentsUnsafe()["test-agent-2"]
	originalExpireAt := originalAgent.ExpireAt
	server.Store().Mu().RUnlock()

	// Wait a bit to ensure time passes
	time.Sleep(10 * time.Millisecond)

	// Send heartbeat
	heartbeatReq := &agentv1.HeartbeatRequest{
		AgentId: "test-agent-2",
	}

	if _, err := server.handleHeartbeatRequest(ctx, heartbeatReq); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	// Verify expire_at was updated in memory
	server.Store().Mu().RLock()
	updatedAgent := server.Store().AgentsUnsafe()["test-agent-2"]
	server.Store().Mu().RUnlock()

	if !updatedAgent.ExpireAt.After(originalExpireAt) {
		t.Error("expire_at should be updated after heartbeat")
	}

	// Wait for async database write
	time.Sleep(100 * time.Millisecond)

	// Verify expire_at was updated in database
	var dbSession registry.AgentSessionDB
	if err := db.Where("agent_id = ?", "test-agent-2").First(&dbSession).Error; err != nil {
		t.Fatalf("failed to query database: %v", err)
	}

	if !dbSession.ExpireAt.After(originalExpireAt) {
		t.Error("database expire_at should be updated after heartbeat")
	}
}

// TestLoadAgentSessionsFromDB tests loading sessions from database
func TestLoadAgentSessionsFromDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&registry.AgentSessionDB{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create agent model and insert a session directly into database
	agentModel := registry.NewAgentSessionModel(db)

	sess := &registry.AgentSession{
		AgentID:  "test-agent-3",
		GameID:   "test-game",
		Env:      "test",
		RPCAddr:  "localhost:19090",
		Version:  "1.0.0",
		ExpireAt: time.Now().Add(1 * time.Hour),
		LastSeen: time.Now(),
		Labels:   map[string]string{"key": "value"},
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true, Version: "1.0"},
		},
		Providers: []registry.ProviderSession{
			{
				ProviderID: "provider1",
				GameID:     "test-game",
				Env:        "test",
				Addr:       "localhost:19091",
				Version:    "1.0.0",
			},
		},
	}

	if err := agentModel.Upsert(context.Background(), sess); err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	// Create a new server (empty registry)
	addrs := []ListenAddr{ParseListenAddr(":0")}
	store := registry.NewStoreWithDB(db)
	agentLoader := registry.NewAgentSessionModel(db)
	server := NewServerWithDB(addrs, store, agentLoader)

	// Load sessions from database
	if err := server.LoadAgentSessions(); err != nil {
		t.Fatalf("LoadAgentSessions failed: %v", err)
	}

	// Verify session was loaded
	server.Store().Mu().RLock()
	loadedAgent := server.Store().AgentsUnsafe()["test-agent-3"]
	server.Store().Mu().RUnlock()

	if loadedAgent == nil {
		t.Fatal("agent not loaded from database")
	}

	if loadedAgent.AgentID != "test-agent-3" {
		t.Errorf("expected agent_id test-agent-3, got %s", loadedAgent.AgentID)
	}

	if loadedAgent.GameID != "test-game" {
		t.Errorf("expected game_id test-game, got %s", loadedAgent.GameID)
	}

	if len(loadedAgent.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(loadedAgent.Providers))
	}

	if len(loadedAgent.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(loadedAgent.Functions))
	}
}

// TestDeleteExpiredSessions tests deletion of expired sessions
func TestDeleteExpiredSessions(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&registry.AgentSessionDB{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create agent model
	agentModel := registry.NewAgentSessionModel(db)

	// Insert an active session
	activeSess := &registry.AgentSession{
		AgentID:  "active-agent",
		GameID:   "test-game",
		Env:      "test",
		RPCAddr:  "localhost:19090",
		Version:  "1.0.0",
		ExpireAt: time.Now().Add(1 * time.Hour),
		LastSeen: time.Now(),
	}

	if err := agentModel.Upsert(context.Background(), activeSess); err != nil {
		t.Fatalf("failed to insert active session: %v", err)
	}

	// Insert an expired session
	expiredSess := &registry.AgentSession{
		AgentID:  "expired-agent",
		GameID:   "test-game",
		Env:      "test",
		RPCAddr:  "localhost:19090",
		Version:  "1.0.0",
		ExpireAt: time.Now().Add(-1 * time.Hour), // Expired
		LastSeen: time.Now().Add(-1 * time.Hour),
	}

	if err := agentModel.Upsert(context.Background(), expiredSess); err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	// Delete expired sessions
	deleted, err := agentModel.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted session, got %d", deleted)
	}

	// Verify active session still exists
	var count int64
	if err := db.Model(&registry.AgentSessionDB{}).Where("agent_id = ?", "active-agent").Count(&count).Error; err != nil {
		t.Fatalf("failed to count active sessions: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 active session, got %d", count)
	}

	// Verify expired session was soft-deleted
	var deletedCount int64
	if err := db.Unscoped().Model(&registry.AgentSessionDB{}).Where("agent_id = ?", "expired-agent").Count(&deletedCount).Error; err != nil {
		t.Fatalf("failed to count deleted sessions: %v", err)
	}

	if deletedCount != 1 {
		t.Errorf("expected 1 deleted session (including soft-deleted), got %d", deletedCount)
	}
}
