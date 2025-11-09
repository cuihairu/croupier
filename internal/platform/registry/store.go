package registry

import (
    "sync"
    "time"
)

// FunctionMeta describes a function capability on an agent.
type FunctionMeta struct {
    Enabled bool
}

// AgentSession represents a registered agent instance in the registry.
type AgentSession struct {
    AgentID  string
    GameID   string
    Env      string
    RPCAddr  string
    Version  string
    Region   string
    Zone     string
    Labels   map[string]string
    Functions map[string]FunctionMeta
    ExpireAt time.Time
}

// Store keeps lightweight agent registry state in-memory.
type Store struct {
    mu     sync.RWMutex
    agents map[string]*AgentSession // agent_id -> session
    // provider capabilities (language-agnostic manifest uploaded via HTTP or Control)
    provCaps map[string]ProviderCaps // provider_id -> caps (latest)
}

func NewStore() *Store { return &Store{agents: map[string]*AgentSession{}, provCaps: map[string]ProviderCaps{}} }

// Mu exposes the lock for read/update operations when callers need batch views.
func (s *Store) Mu() *sync.RWMutex { return &s.mu }

// AgentsUnsafe returns the internal agents map without copying. Callers MUST hold Mu().RLock/Lock.
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }

// UpsertAgent inserts or updates an agent session by AgentID.
func (s *Store) UpsertAgent(a *AgentSession) {
    if a == nil || a.AgentID == "" { return }
    s.mu.Lock()
    defer s.mu.Unlock()
    cur := s.agents[a.AgentID]
    if cur == nil {
        s.agents[a.AgentID] = a
        return
    }
    // merge minimal fields
    cur.GameID, cur.Env, cur.RPCAddr, cur.Version = a.GameID, a.Env, a.RPCAddr, a.Version
    cur.Region, cur.Zone = a.Region, a.Zone
    if a.Labels != nil { cur.Labels = a.Labels }
    if a.Functions != nil { cur.Functions = a.Functions }
    if !a.ExpireAt.IsZero() { cur.ExpireAt = a.ExpireAt }
}

// ProviderCaps represents a provider manifest snapshot registered at runtime.
type ProviderCaps struct {
    ID       string
    Version  string
    Lang     string
    SDK      string
    Manifest []byte // raw JSON
    UpdatedAt time.Time
}

// UpsertProviderCaps inserts or updates provider capabilities by provider ID.
func (s *Store) UpsertProviderCaps(c ProviderCaps) {
    if c.ID == "" || len(c.Manifest) == 0 { return }
    s.mu.Lock(); defer s.mu.Unlock()
    c.UpdatedAt = time.Now()
    s.provCaps[c.ID] = c
}

// ListProviderCaps returns a snapshot of provider capabilities.
func (s *Store) ListProviderCaps() []ProviderCaps {
    s.mu.RLock(); defer s.mu.RUnlock()
    out := make([]ProviderCaps, 0, len(s.provCaps))
    for _, v := range s.provCaps { out = append(out, v) }
    return out
}
