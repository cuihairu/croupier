package registry

import "sync"

type Entry struct {
    Addr    string
    Version string
}

// LocalStore maps function id to local game-server endpoint and version.
type LocalStore struct {
    mu sync.RWMutex
    fn map[string]Entry
}

func NewLocalStore() *LocalStore { return &LocalStore{fn: map[string]Entry{}} }

func (s *LocalStore) Set(fid, addr, version string) {
    s.mu.Lock(); defer s.mu.Unlock()
    s.fn[fid] = Entry{Addr: addr, Version: version}
}

func (s *LocalStore) Get(fid string) (Entry, bool) {
    s.mu.RLock(); defer s.mu.RUnlock()
    v, ok := s.fn[fid]
    return v, ok
}

func (s *LocalStore) List() map[string]Entry {
    s.mu.RLock(); defer s.mu.RUnlock()
    out := make(map[string]Entry, len(s.fn))
    for k, v := range s.fn { out[k] = v }
    return out
}
