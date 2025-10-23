package registry

import (
    "sync"
    "time"
)

// Instance represents a local game-server instance that can handle a function.
type Instance struct {
    ServiceID string
    Addr      string
    Version   string
    LastSeen  time.Time
}

// LocalStore maps function id to a list of local instances, with round-robin routing.
type LocalStore struct {
    mu sync.RWMutex
    fn map[string][]Instance
    rr map[string]int
}

func NewLocalStore() *LocalStore { return &LocalStore{fn: map[string][]Instance{}, rr: map[string]int{}} }

// Add appends or updates an instance for a function id.
func (s *LocalStore) Add(fid, serviceID, addr, version string) {
    s.mu.Lock(); defer s.mu.Unlock()
    lst := s.fn[fid]
    // dedup by serviceID+addr
    for i := range lst {
        if lst[i].ServiceID == serviceID && lst[i].Addr == addr {
            lst[i].Version = version
            lst[i].LastSeen = time.Now()
            s.fn[fid] = lst
            return
        }
    }
    lst = append(lst, Instance{ServiceID: serviceID, Addr: addr, Version: version, LastSeen: time.Now()})
    s.fn[fid] = lst
}

// Next returns the next instance for a function id using round-robin.
func (s *LocalStore) Next(fid string) (Instance, bool) {
    s.mu.Lock(); defer s.mu.Unlock()
    lst := s.fn[fid]
    if len(lst) == 0 { return Instance{}, false }
    idx := s.rr[fid] % len(lst)
    s.rr[fid] = (s.rr[fid] + 1) % len(lst)
    return lst[idx], true
}

// List returns a copy of the mapping for summary purposes.
func (s *LocalStore) List() map[string][]Instance {
    s.mu.RLock(); defer s.mu.RUnlock()
    out := make(map[string][]Instance, len(s.fn))
    for k, v := range s.fn {
        cp := make([]Instance, len(v))
        copy(cp, v)
        out[k] = cp
    }
    return out
}

// Instances returns a copy of instances for a function id.
func (s *LocalStore) Instances(fid string) []Instance {
    s.mu.RLock(); defer s.mu.RUnlock()
    lst := s.fn[fid]
    out := make([]Instance, len(lst))
    copy(out, lst)
    return out
}

// TouchByService updates LastSeen for instances matching serviceID (and optional addr).
func (s *LocalStore) TouchByService(serviceID, addr string) {
    s.mu.Lock(); defer s.mu.Unlock()
    now := time.Now()
    for fid, lst := range s.fn {
        changed := false
        for i := range lst {
            if lst[i].ServiceID == serviceID && (addr == "" || lst[i].Addr == addr) {
                lst[i].LastSeen = now
                changed = true
            }
        }
        if changed { s.fn[fid] = lst }
    }
}

// Prune removes instances whose LastSeen is older than ttl.
func (s *LocalStore) Prune(ttl time.Duration) int {
    s.mu.Lock(); defer s.mu.Unlock()
    cutoff := time.Now().Add(-ttl)
    removed := 0
    for fid, lst := range s.fn {
        keep := lst[:0]
        for _, inst := range lst {
            if inst.LastSeen.After(cutoff) { keep = append(keep, inst) } else { removed++ }
        }
        if len(keep) == 0 { delete(s.fn, fid); delete(s.rr, fid) } else { s.fn[fid] = keep }
    }
    return removed
}
