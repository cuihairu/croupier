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
    // index function id to agent ids
    fIndex  map[string]map[string]struct{}
}

func NewStore() *Store {
    return &Store{agents: map[string]*AgentSession{}, fIndex: map[string]map[string]struct{}{}}
}

func (s *Store) UpsertAgent(sess *AgentSession) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // remove previous index entries for this agent
    if old := s.agents[sess.AgentID]; old != nil {
        for fid := range old.Functions {
            if m := s.fIndex[fid]; m != nil { delete(m, old.AgentID) }
        }
    }
    s.agents[sess.AgentID] = sess
    // rebuild function index entries for this agent
    for fid := range sess.Functions {
        m := s.fIndex[fid]
        if m == nil { m = map[string]struct{}{}; s.fIndex[fid] = m }
        m[sess.AgentID] = struct{}{}
    }
}

func (s *Store) AgentsForFunction(fid string) []*AgentSession {
    s.mu.RLock()
    defer s.mu.RUnlock()
    ids := s.fIndex[fid]
    var out []*AgentSession
    for id := range ids {
        if a := s.agents[id]; a != nil && time.Now().Before(a.ExpireAt) {
            out = append(out, a)
        }
    }
    return out
}
