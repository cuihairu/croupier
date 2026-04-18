package jobs

import "sync"

// Router maps job_id to agentID for streaming and cancellation routing.
type Router struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewRouter() *Router { return &Router{m: map[string]string{}} }

func (r *Router) Set(jobID, agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[jobID] = agentID
}

func (r *Router) Get(jobID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.m[jobID]
	return v, ok
}
