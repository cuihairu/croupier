package tasks

import "sync"

type Registry struct {
	mu    sync.RWMutex
	items map[string]*Runtime
}

func NewRegistry() *Registry {
	return &Registry{items: make(map[string]*Runtime)}
}

func (r *Registry) Set(taskID string, runtime *Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[taskID] = runtime
}

func (r *Registry) Get(taskID string) (*Runtime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[taskID]
	return item, ok
}

func (r *Registry) Delete(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, taskID)
}
