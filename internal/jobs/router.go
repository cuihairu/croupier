package jobs

import "sync"

// Router maps job_id to agent RPC address for streaming.
type Router struct {
    mu   sync.RWMutex
    m    map[string]string
}

func NewRouter() *Router { return &Router{m: map[string]string{}} }

func (r *Router) Set(jobID, rpcAddr string) {
    r.mu.Lock(); defer r.mu.Unlock()
    r.m[jobID] = rpcAddr
}

func (r *Router) Get(jobID string) (string, bool) {
    r.mu.RLock(); defer r.mu.RUnlock()
    v, ok := r.m[jobID]
    return v, ok
}

