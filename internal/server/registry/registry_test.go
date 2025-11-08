package registry

import (
    "testing"
    "time"
)

// Test that UpsertAgent writes both scoped and legacy indexes correctly
func TestStore_UpsertAndLookupScoped(t *testing.T) {
    s := NewStore()
    sess := &AgentSession{
        AgentID:   "a1",
        Version:   "0.1.0",
        RPCAddr:   "127.0.0.1:1234",
        GameID:    "game_x",
        Env:       "dev",
        Functions: map[string]FunctionMeta{
            "player.ban": {Enabled: true, Entity: "player", Operation: "ban"},
            "player.kick": {Enabled: true, Entity: "player", Operation: "kick"},
        },
        ExpireAt:  time.Now().Add(time.Hour),
    }
    s.UpsertAgent(sess)

    // scoped lookup should find the agent
    got := s.AgentsForFunctionScoped("game_x", "player.ban", false)
    if len(got) != 1 || got[0].AgentID != "a1" {
        t.Fatalf("expected agent a1 in scoped lookup, got %#v", got)
    }

    // different game should not find when fallback=false
    got = s.AgentsForFunctionScoped("game_y", "player.ban", false)
    if len(got) != 0 {
        t.Fatalf("expected no agents for other game, got %#v", got)
    }

    // fallback to legacy index should return the agent
    got = s.AgentsForFunctionScoped("game_y", "player.ban", true)
    if len(got) != 1 || got[0].AgentID != "a1" {
        t.Fatalf("expected fallback to legacy to return a1, got %#v", got)
    }
}
