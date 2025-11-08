package registry

import (
    "sync"
    "time"
)

// FunctionMeta contains metadata about a function registration
type FunctionMeta struct {
    Entity    string // Entity type this function operates on (e.g., "item", "player")
    Operation string // Operation type (e.g., "create", "read", "update", "delete")
    Enabled   bool   // Whether this function is currently enabled
}

type AgentSession struct {
    AgentID   string
    Version   string
    RPCAddr   string
    GameID    string
    Env       string
    Region    string
    Zone      string
    Labels    map[string]string
    Functions map[string]FunctionMeta // Upgraded from map[string]bool
    ExpireAt  time.Time
}

type Store struct {
    mu      sync.RWMutex
    agents  map[string]*AgentSession
    // index (game_id|function_id) -> agent ids
    fIndex  map[string]map[string]struct{}
    // new entity/operation indexes
    entityIndex    map[string]map[string]struct{} // entity -> function_ids
    operationIndex map[string]map[string]struct{} // operation -> function_ids
}

func NewStore() *Store {
    return &Store{
        agents:         map[string]*AgentSession{},
        fIndex:         map[string]map[string]struct{}{},
        entityIndex:    map[string]map[string]struct{}{},
        operationIndex: map[string]map[string]struct{}{},
    }
}

func (s *Store) UpsertAgent(sess *AgentSession) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // remove previous index entries for this agent
    if old := s.agents[sess.AgentID]; old != nil {
        for fid, meta := range old.Functions {
            key := compositeKey(old.GameID, fid)
            if m := s.fIndex[key]; m != nil { delete(m, old.AgentID) }

            // Remove from entity/operation indexes
            if meta.Entity != "" {
                if m := s.entityIndex[meta.Entity]; m != nil {
                    delete(m, fid)
                }
            }
            if meta.Operation != "" {
                if m := s.operationIndex[meta.Operation]; m != nil {
                    delete(m, fid)
                }
            }
        }
    }
    s.agents[sess.AgentID] = sess
    // rebuild function index entries for this agent
    for fid, meta := range sess.Functions {
        // index by (game_id|function_id) for scoped routing
        key := compositeKey(sess.GameID, fid)
        m := s.fIndex[key]
        if m == nil {
            m = map[string]struct{}{}
            s.fIndex[key] = m
        }
        m[sess.AgentID] = struct{}{}
        // maintain legacy index by function only as a fallback for older callers
        legacy := s.fIndex[fid]
        if legacy == nil {
            legacy = map[string]struct{}{}
            s.fIndex[fid] = legacy
        }
        legacy[sess.AgentID] = struct{}{}

        // Add to entity/operation indexes
        if meta.Entity != "" {
            if s.entityIndex[meta.Entity] == nil {
                s.entityIndex[meta.Entity] = map[string]struct{}{}
            }
            s.entityIndex[meta.Entity][fid] = struct{}{}
        }
        if meta.Operation != "" {
            if s.operationIndex[meta.Operation] == nil {
                s.operationIndex[meta.Operation] = map[string]struct{}{}
            }
            s.operationIndex[meta.Operation][fid] = struct{}{}
        }
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

// GetFunctionsForEntity returns all function IDs that operate on the given entity
func (s *Store) GetFunctionsForEntity(entity string) []string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var functions []string
    if fids := s.entityIndex[entity]; fids != nil {
        for fid := range fids {
            functions = append(functions, fid)
        }
    }
    return functions
}

// GetEntitiesWithOperation returns all entities that support the given operation
func (s *Store) GetEntitiesWithOperation(operation string) []string {
    s.mu.RLock()
    defer s.mu.RUnlock()

    entities := make(map[string]struct{})
    if fids := s.operationIndex[operation]; fids != nil {
        // For each function with this operation, find its entity
        for _, agent := range s.agents {
            for fid, meta := range agent.Functions {
                if _, hasFid := fids[fid]; hasFid && meta.Entity != "" {
                    entities[meta.Entity] = struct{}{}
                }
            }
        }
    }

    var result []string
    for entity := range entities {
        result = append(result, entity)
    }
    return result
}

// GetFunctionByEntityOp returns function ID for the given entity and operation combination
func (s *Store) GetFunctionByEntityOp(entity, operation string) string {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Look through all agents to find function matching both entity and operation
    for _, agent := range s.agents {
        for fid, meta := range agent.Functions {
            if meta.Entity == entity && meta.Operation == operation && meta.Enabled {
                return fid
            }
        }
    }
    return ""
}

// Introspection helpers (for HTTP)
func (s *Store) Mu() *sync.RWMutex { return &s.mu }
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }
