package registry

import (
    "sync"
    "time"
)

type AgentSession struct {
    AgentID   string
    Version   string
    RPCAddr   string
    GameID    string
    Env       string
    Functions map[string]bool
    ExpireAt  time.Time
}

type Store struct {
    mu      sync.RWMutex
    agents  map[string]*AgentSession
    // index (game_id|function_id) -> agent ids
    fIndex  map[string]map[string]struct{}
}

func NewStore() *Store { return &Store{agents: map[string]*AgentSession{}, fIndex: map[string]map[string]struct{}{}} }

func (s *Store) UpsertAgent(sess *AgentSession) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // remove previous index entries for this agent
    if old := s.agents[sess.AgentID]; old != nil {
        for fid := range old.Functions {
            key := compositeKey(old.GameID, fid)
            if m := s.fIndex[key]; m != nil { delete(m, old.AgentID) }
        }
    }
    s.agents[sess.AgentID] = sess
    // rebuild function index entries for this agent
    for fid := range sess.Functions {
        key := compositeKey(sess.GameID, fid)
        m := s.fIndex[key]
        if m == nil { m = map[string]struct{}{}; s.fIndex[fid] = m }
        m[sess.AgentID] = struct{}{}
    }
}

func (s *Store) AgentsForFunction(fid string) []*AgentSession {
    s.mu.RLock()
    defer s.mu.RUnlock()
    ids := s.fIndex[fid] // legacy without game scope (unlikely populated)
    var out []*AgentSession
    for id := range ids {
        if a := s.agents[id]; a != nil && time.Now().Before(a.ExpireAt) {
            out = append(out, a)
        }
    }
    return out
}

// AgentsForFunctionScoped returns agents registered for (game_id,function_id), with optional fallback.
func (s *Store) AgentsForFunctionScoped(gameID, fid string, fallback bool) []*AgentSession {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var out []*AgentSession
    // primary: game-scoped key
    if ids := s.fIndex[compositeKey(gameID, fid)]; ids != nil {
        for id := range ids {
            if a := s.agents[id]; a != nil && time.Now().Before(a.ExpireAt) {
                out = append(out, a)
            }
        }
        if len(out) > 0 || !fallback { return out }
    }
    // fallback: any game (legacy)
    if fallback {
        if ids := s.fIndex[fid]; ids != nil {
            for id := range ids {
                if a := s.agents[id]; a != nil && time.Now().Before(a.ExpireAt) {
                    out = append(out, a)
                }
            }
        }
    }
    return out
}

func compositeKey(gameID, fid string) string { return gameID + "|" + fid }

// Introspection helpers (for HTTP)
func (s *Store) Mu() *sync.RWMutex { return &s.mu }
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }
