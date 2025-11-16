package registry

import (
    "encoding/json"
    "io/fs"
    "os"
    "path/filepath"
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

// BuildUnifiedDescriptors merges all provider manifests into a unified descriptor structure
func (s *Store) BuildUnifiedDescriptors() map[string]interface{} {
    s.mu.RLock()
    defer s.mu.RUnlock()

    unified := map[string]interface{}{
        "providers":  make(map[string]interface{}),
        "functions":  make([]interface{}, 0),
        "entities":   make([]interface{}, 0),
        "operations": make([]interface{}, 0),
    }

    for providerID, provCaps := range s.provCaps {
        if len(provCaps.Manifest) == 0 {
            continue
        }

        // Parse the manifest JSON
        var manifest map[string]interface{}
        if err := json.Unmarshal(provCaps.Manifest, &manifest); err != nil {
            continue // skip invalid manifests
        }

        // Add provider info
        if providers, ok := unified["providers"].(map[string]interface{}); ok {
            providers[providerID] = map[string]interface{}{
                "id":      provCaps.ID,
                "version": provCaps.Version,
                "lang":    provCaps.Lang,
                "sdk":     provCaps.SDK,
                "updated_at": provCaps.UpdatedAt,
            }
            if provider, exists := manifest["provider"]; exists {
                providers[providerID] = provider
            }
        }

        // Merge functions
        if functions, exists := manifest["functions"]; exists {
            if funcList, ok := functions.([]interface{}); ok {
                if unifiedFunctions, ok := unified["functions"].([]interface{}); ok {
                    unified["functions"] = append(unifiedFunctions, funcList...)
                }
            }
        }

        // Merge entities
        if entities, exists := manifest["entities"]; exists {
            if entityList, ok := entities.([]interface{}); ok {
                if unifiedEntities, ok := unified["entities"].([]interface{}); ok {
                    unified["entities"] = append(unifiedEntities, entityList...)
                }
            }
        }

        // Merge operations
        if operations, exists := manifest["operations"]; exists {
            if opList, ok := operations.([]interface{}); ok {
                if unifiedOperations, ok := unified["operations"].([]interface{}); ok {
                    unified["operations"] = append(unifiedOperations, opList...)
                }
            }
        }
    }

    return unified
}

// BuildFunctionIndex parses provider manifests and builds an index of function metadata by id.
// Expected manifest structure for functions:
// {
//   "functions": [
//     {
//       "id": "player.ban",
//       "display_name": {"en":"Ban Player","zh":"封禁玩家"},
//       "summary": {"en":"...","zh":"..."},
//       "tags": ["moderation","player"],
//       "menu": { "section":"Function Management","group":"Moderation","path":"/functions/invoke","order":120,"icon":"StopOutlined","badge":"beta","hidden":false },
//       "permissions": {
//          "verbs": ["read","invoke","view_history"],
//          "scopes": ["game","env","function_id"],
//          "defaults": [ {"role":"operator","verbs":["invoke"]}, {"role":"auditor","verbs":["read","view_history"]} ],
//          "i18n_zh": { "invoke":"调用函数 封禁玩家" }
//       }
//     }
//   ]
// }
func (s *Store) BuildFunctionIndex() map[string]map[string]interface{} {
    s.mu.RLock()
    defer s.mu.RUnlock()
    idx := map[string]map[string]interface{}{}
    for _, pc := range s.provCaps {
        if len(pc.Manifest) == 0 {
            continue
        }
        var m map[string]interface{}
        if err := json.Unmarshal(pc.Manifest, &m); err != nil {
            continue
        }
        rawFns, ok := m["functions"]
        if !ok {
            continue
        }
        fnList, ok := rawFns.([]interface{})
        if !ok {
            continue
        }
        for _, it := range fnList {
            fn, ok := it.(map[string]interface{})
            if !ok {
                continue
            }
            idv, ok := fn["id"]
            if !ok {
                continue
            }
            fid, _ := idv.(string)
            if fid == "" {
                continue
            }
            // Shallow copy metadata into index
            meta := map[string]interface{}{}
            if dn, ok := fn["display_name"]; ok {
                meta["display_name"] = dn
            }
            if sm, ok := fn["summary"]; ok {
                meta["summary"] = sm
            }
            if tags, ok := fn["tags"]; ok {
                meta["tags"] = tags
            }
            if menu, ok := fn["menu"]; ok {
                meta["menu"] = menu
            }
            if perm, ok := fn["permissions"]; ok {
                meta["permissions"] = perm
            }
            if len(meta) > 0 {
                idx[fid] = meta
            }
        }
    }
    return idx
}

// LoadUIOverrides loads server-side UI/RBAC overrides from one or more JSON files.
// Expected structure:
// {
//   "player.ban": {
//     "display_name": {"zh":"封禁玩家","en":"Ban Player"},
//     "summary": {"zh":"..."},
//     "tags": ["moderation"],
//     "menu": { "section":"Function Management", "group":"Moderation", "order": 100, "hidden": false },
//     "permissions": { "verbs":["read","invoke"], "scopes":["game","env","function_id"], "defaults":[{"role":"operator","verbs":["invoke"]}] }
//   },
//   ...
// }
func (s *Store) LoadUIOverrides(paths ...string) map[string]map[string]interface{} {
    merged := map[string]map[string]interface{}{}
    for _, p := range paths {
        if p == "" {
            continue
        }
        expanded := expandIfDir(p)
        for _, file := range expanded {
            b, err := os.ReadFile(file)
            if err != nil || len(b) == 0 {
                continue
            }
            var m map[string]map[string]interface{}
            if err := json.Unmarshal(b, &m); err != nil {
                continue
            }
            for fid, meta := range m {
                if fid == "" || meta == nil {
                    continue
                }
                if _, ok := merged[fid]; !ok {
                    merged[fid] = map[string]interface{}{}
                }
                // shallow merge/replace: server overrides take precedence
                for k, v := range meta {
                    merged[fid][k] = v
                }
            }
        }
    }
    return merged
}

// expandIfDir: if p is a directory, return *.json files within; else return [p] if exists; else [].
func expandIfDir(p string) []string {
    info, err := os.Stat(p)
    if err != nil {
        return nil
    }
    if info.IsDir() {
        files := []string{}
        _ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                return nil
            }
            if d.IsDir() {
                return nil
            }
            if filepath.Ext(d.Name()) == ".json" {
                files = append(files, path)
            }
            return nil
        })
        return files
    }
    return []string{p}
}
