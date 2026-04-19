package tasks

import (
	"context"
	"sync"
)

type Runtime struct {
	taskID string
	ctx    context.Context
	cancel context.CancelFunc
	store  *Store

	mu              sync.RWMutex
	cancelRequested bool
}

func (r *Runtime) Context() context.Context { return r.ctx }
func (r *Runtime) TaskID() string           { return r.taskID }

func (r *Runtime) ReportProgress(progress int32, message string, payload []byte) error {
	return r.store.AppendEvent(r.ctx, r.taskID, EventProgress, progress, message, payload)
}

func (r *Runtime) Log(message string, payload []byte) error {
	return r.store.AppendEvent(r.ctx, r.taskID, EventLog, 0, message, payload)
}

func (r *Runtime) IsCancelled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cancelRequested
}

func (r *Runtime) RequestCancel() {
	r.mu.Lock()
	r.cancelRequested = true
	r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}
