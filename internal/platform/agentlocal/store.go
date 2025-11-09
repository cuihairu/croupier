package agentlocal

import (
    "sync"
    "time"
)

type Instance struct {
    ServiceID string
    Addr      string
    Version   string
    LastSeen  time.Time
}

type LocalStore struct {
    mu   sync.RWMutex
    // function_id -> instances
    data map[string][]Instance
}

func NewLocalStore() *LocalStore { return &LocalStore{data: map[string][]Instance{}} }

// Register replaces instances for the provided function ids for a service.
func (s *LocalStore) Register(serviceID, addr, version string, fnIDs []string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now()
    // remove prior instances from this serviceID for all functions
    for fid, arr := range s.data {
        next := arr[:0]
        for _, it := range arr {
            if it.ServiceID != serviceID { next = append(next, it) }
        }
        if len(next) == 0 { delete(s.data, fid) } else { s.data[fid] = next }
    }
    inst := Instance{ServiceID: serviceID, Addr: addr, Version: version, LastSeen: now}
    for _, fid := range fnIDs {
        s.data[fid] = append(s.data[fid], inst)
    }
}

// Heartbeat updates last seen for a service across all functions.
func (s *LocalStore) Heartbeat(serviceID string) {
    s.mu.Lock(); defer s.mu.Unlock()
    now := time.Now()
    for fid, arr := range s.data {
        for i := range arr {
            if arr[i].ServiceID == serviceID { arr[i].LastSeen = now }
        }
        s.data[fid] = arr
    }
}

// List snapshot of functions and instances.
func (s *LocalStore) List() map[string][]Instance {
    s.mu.RLock(); defer s.mu.RUnlock()
    out := make(map[string][]Instance, len(s.data))
    for fid, arr := range s.data {
        cp := make([]Instance, len(arr))
        copy(cp, arr)
        out[fid] = cp
    }
    return out
}

// Prune removes instances older than maxAge; returns removed count.
func (s *LocalStore) Prune(maxAge time.Duration) int {
    s.mu.Lock(); defer s.mu.Unlock()
    now := time.Now()
    removed := 0
    for fid, arr := range s.data {
        next := arr[:0]
        for _, it := range arr {
            if now.Sub(it.LastSeen) <= maxAge { next = append(next, it) } else { removed++ }
        }
        if len(next) == 0 { delete(s.data, fid) } else { s.data[fid] = next }
    }
    return removed
}

