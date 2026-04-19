package agent

import "sync"

// taskIndex maintains taskID -> instance address mapping for routing cancellation and result queries.
type taskIndex struct {
	mu   sync.RWMutex
	byID map[string]string
}

func newTaskIndex() *taskIndex { return &taskIndex{byID: map[string]string{}} }

func (t *taskIndex) Set(taskID, addr string) {
	if taskID == "" || addr == "" {
		return
	}
	t.mu.Lock()
	t.byID[taskID] = addr
	t.mu.Unlock()
}

func (t *taskIndex) Get(taskID string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	addr, ok := t.byID[taskID]
	return addr, ok
}

func (t *taskIndex) Delete(taskID string) {
	if taskID == "" {
		return
	}
	t.mu.Lock()
	delete(t.byID, taskID)
	t.mu.Unlock()
}

func (t *taskIndex) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.byID)
}
