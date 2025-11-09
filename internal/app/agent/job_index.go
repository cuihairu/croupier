package agent

import "sync"

// jobIndex maintains jobID -> instance address mapping for routing CANCEL and result queries.
type jobIndex struct {
    mu   sync.RWMutex
    byID map[string]string
}

func newJobIndex() *jobIndex { return &jobIndex{byID: map[string]string{}} }

func (j *jobIndex) Set(jobID, addr string) {
    if jobID == "" || addr == "" { return }
    j.mu.Lock(); j.byID[jobID] = addr; j.mu.Unlock()
}

func (j *jobIndex) Get(jobID string) (string, bool) {
    j.mu.RLock(); defer j.mu.RUnlock()
    addr, ok := j.byID[jobID]
    return addr, ok
}

func (j *jobIndex) Delete(jobID string) {
    if jobID == "" { return }
    j.mu.Lock(); delete(j.byID, jobID); j.mu.Unlock()
}

func (j *jobIndex) Len() int {
    j.mu.RLock(); defer j.mu.RUnlock()
    return len(j.byID)
}
