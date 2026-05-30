// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package server

import (
	"sync"
	"testing"
	"time"
)

func TestAgentSessionFields(t *testing.T) {
	sess := &AgentSession{
		AgentID:     "agent-123",
		SessionID:   "session-456",
		GameID:      "game-789",
		Env:         "production",
		Version:     "1.0.0",
		RPCAddr:     "127.0.0.1:50051",
		ConnectedAt: time.Now(),
	}

	if sess.AgentID != "agent-123" {
		t.Errorf("AgentID = %s, want agent-123", sess.AgentID)
	}
	if sess.SessionID != "session-456" {
		t.Errorf("SessionID = %s, want session-456", sess.SessionID)
	}
	if sess.GameID != "game-789" {
		t.Errorf("GameID = %s, want game-789", sess.GameID)
	}
	if sess.Env != "production" {
		t.Errorf("Env = %s, want production", sess.Env)
	}
	if sess.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", sess.Version)
	}
	if sess.RPCAddr != "127.0.0.1:50051" {
		t.Errorf("RPCAddr = %s, want 127.0.0.1:50051", sess.RPCAddr)
	}
}

func TestAgentSessionLastSeen(t *testing.T) {
	sess := &AgentSession{}

	// Initially zero
	if ts := sess.LastSeen.Load(); ts != 0 {
		t.Errorf("Initial LastSeen = %d, want 0", ts)
	}

	// Update last seen
	sess.UpdateLastSeen()

	lastSeen := sess.GetLastSeen()
	if time.Since(lastSeen) > time.Second {
		t.Errorf("GetLastSeen() = %v, too far in the past", lastSeen)
	}
}

func TestAgentSessionClose(t *testing.T) {
	// Nil connection should not panic
	sess := &AgentSession{}
	if err := sess.Close(); err != nil {
		t.Errorf("Close() with nil conn returned error: %v", err)
	}
}

func TestNewAgentSessionStore(t *testing.T) {
	store := NewAgentSessionStore()
	if store == nil {
		t.Fatal("NewAgentSessionStore() returned nil")
	}
	if store.sessions == nil {
		t.Error("NewAgentSessionStore() sessions map is nil")
	}
	if store.Count() != 0 {
		t.Errorf("NewAgentSessionStore() Count() = %d, want 0", store.Count())
	}
}

func TestAgentSessionStoreAddAndGet(t *testing.T) {
	store := NewAgentSessionStore()
	sess := &AgentSession{
		AgentID:   "agent-1",
		SessionID: "session-1",
		GameID:    "game-1",
		Env:       "dev",
	}

	// Add session
	err := store.Add(sess)
	if err != nil {
		t.Errorf("Add() error = %v", err)
	}

	if store.Count() != 1 {
		t.Errorf("Count() after Add() = %d, want 1", store.Count())
	}

	// Get session
	got, ok := store.Get("agent-1")
	if !ok {
		t.Error("Get() returned not found")
	}
	if got != sess {
		t.Errorf("Get() = %v, want %v", got, sess)
	}

	// Get non-existent
	_, ok = store.Get("non-existent")
	if ok {
		t.Error("Get() for non-existent agent found something")
	}
}

func TestAgentSessionStoreAddDuplicate(t *testing.T) {
	store := NewAgentSessionStore()
	sess1 := &AgentSession{
		AgentID:   "agent-1",
		SessionID: "session-1",
	}
	sess2 := &AgentSession{
		AgentID:   "agent-1", // Same agent ID
		SessionID: "session-2",
	}

	err := store.Add(sess1)
	if err != nil {
		t.Errorf("First Add() error = %v", err)
	}

	err = store.Add(sess2)
	if err == nil {
		t.Error("Second Add() with duplicate agent ID should return error")
	}
}

func TestAgentSessionStoreUpsert(t *testing.T) {
	store := NewAgentSessionStore()
	sess1 := &AgentSession{
		AgentID:   "agent-1",
		SessionID: "session-1",
		Env:       "dev",
	}
	sess2 := &AgentSession{
		AgentID:   "agent-1", // Same agent ID
		SessionID: "session-2",
		Env:       "prod",
	}

	// Upsert should add first
	store.Upsert(sess1)
	if store.Count() != 1 {
		t.Errorf("Count() after first Upsert() = %d, want 1", store.Count())
	}

	// Upsert should replace
	store.Upsert(sess2)
	if store.Count() != 1 {
		t.Errorf("Count() after second Upsert() = %d, want 1", store.Count())
	}

	got, ok := store.Get("agent-1")
	if !ok {
		t.Error("Get() after upsert returned not found")
	}
	if got.SessionID != "session-2" {
		t.Errorf("SessionID after upsert = %s, want session-2", got.SessionID)
	}
	if got.Env != "prod" {
		t.Errorf("Env after upsert = %s, want prod", got.Env)
	}
}

func TestAgentSessionStoreRemove(t *testing.T) {
	store := NewAgentSessionStore()
	sess := &AgentSession{
		AgentID:   "agent-1",
		SessionID: "session-1",
	}

	store.Add(sess)
	if store.Count() != 1 {
		t.Errorf("Count() before Remove() = %d, want 1", store.Count())
	}

	store.Remove("agent-1")
	if store.Count() != 0 {
		t.Errorf("Count() after Remove() = %d, want 0", store.Count())
	}

	// Remove non-existent should not panic
	store.Remove("non-existent")
	if store.Count() != 0 {
		t.Errorf("Count() after removing non-existent = %d, want 0", store.Count())
	}
}

func TestAgentSessionStoreList(t *testing.T) {
	store := NewAgentSessionStore()

	// Empty list
	list := store.List()
	if len(list) != 0 {
		t.Errorf("List() on empty store = %v, want empty", list)
	}

	// Add some sessions
	store.Add(&AgentSession{AgentID: "agent-1"})
	store.Add(&AgentSession{AgentID: "agent-2"})
	store.Add(&AgentSession{AgentID: "agent-3"})

	list = store.List()
	if len(list) != 3 {
		t.Errorf("List() length = %d, want 3", len(list))
	}
}

func TestAgentSessionStorePruneStale(t *testing.T) {
	store := NewAgentSessionStore()

	// Create sessions with different LastSeen times
	now := time.Now()

	oldSess := &AgentSession{
		AgentID: "old-agent",
	}
	newSess := &AgentSession{
		AgentID: "new-agent",
	}

	// Add sessions (Upsert calls UpdateLastSeen)
	store.Upsert(oldSess)
	store.Upsert(newSess)

	// Manually set old session's LastSeen to the past
	oldSess.LastSeen.Store(now.Add(-2 * time.Hour).Unix())

	if store.Count() != 2 {
		t.Errorf("Count() before PruneStale() = %d, want 2", store.Count())
	}

	// Prune sessions older than 1 hour
	pruned := store.PruneStale(1 * time.Hour)
	if pruned != 1 {
		t.Errorf("PruneStale() returned %d, want 1", pruned)
	}
	if store.Count() != 1 {
		t.Errorf("Count() after PruneStale() = %d, want 1", store.Count())
	}

	// Verify new session still exists
	_, ok := store.Get("new-agent")
	if !ok {
		t.Error("new-agent should still exist after pruning")
	}

	// Verify old session was removed
	_, ok = store.Get("old-agent")
	if ok {
		t.Error("old-agent should have been pruned")
	}
}

func TestAgentSessionStoreConcurrentAccess(t *testing.T) {
	store := NewAgentSessionStore()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sess := &AgentSession{
				AgentID:   string(rune('a'+n%26)) + "-agent",
				SessionID: "session",
			}
			store.Upsert(sess)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agentID := string(rune('a'+n%26)) + "-agent"
			store.Get(agentID)
		}(i)
	}

	// Concurrent removes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agentID := string(rune('a'+n%26)) + "-agent"
			store.Remove(agentID)
		}(i)
	}

	wg.Wait()

	// Store should still be functional
	sess := &AgentSession{AgentID: "final-agent"}
	store.Upsert(sess)
	if store.Count() == 0 {
		t.Error("Store has no sessions after concurrent operations")
	}
}

func TestAgentSessionStoreResolveAgentConn(t *testing.T) {
	store := NewAgentSessionStore()

	// No session initially
	_, ok := store.ResolveAgentConn("agent-1")
	if ok {
		t.Error("ResolveAgentConn() found session that doesn't exist")
	}

	// Add session without connection
	sess := &AgentSession{AgentID: "agent-1"}
	store.Upsert(sess)

	conn, ok := store.ResolveAgentConn("agent-1")
	if ok {
		t.Error("ResolveAgentConn() should return false when session has no conn")
	}
	if conn != nil {
		t.Error("ResolveAgentConn() should return nil conn when session has no conn")
	}

	// Add session with nil conn explicitly
	sess.conn = nil
	store.Upsert(sess)
	conn, ok = store.ResolveAgentConn("agent-1")
	if ok {
		t.Error("ResolveAgentConn() should return false when conn is nil")
	}
}

func TestSessionResolverAdapter(t *testing.T) {
	store := NewAgentSessionStore()
	adapter := NewSessionResolverAdapter(store)

	if adapter == nil {
		t.Fatal("NewSessionResolverAdapter() returned nil")
	}
	if adapter.store != store {
		t.Error("Adapter store field doesn't match provided store")
	}
}
