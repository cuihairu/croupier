package jobs

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"

    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    "github.com/cuihairu/croupier/internal/transport/interceptors"
    "google.golang.org/grpc"
)

type Executor struct {
    mu    sync.RWMutex
    ch    map[string]chan *functionv1.JobEvent
    idem  map[string]idemEntry // idempotency key -> entry
    cancel map[string]context.CancelFunc
    results map[string]JobStatus // job_id -> final status
}

func NewExecutor() *Executor { return &Executor{ch: map[string]chan *functionv1.JobEvent{}, idem: map[string]idemEntry{}, cancel: map[string]context.CancelFunc{}, results: map[string]JobStatus{}} }

// idemEntry stores a job id and its expiry for idempotency window.
type idemEntry struct {
    jobID   string
    expireAt time.Time
}

const idemWindow = 10 * time.Minute

// JobStatus reflects current/final state of a job.
type JobStatus struct {
    State       string    // "running" | "done" | "error"
    Payload     []byte    // final result when done
    Error       string    // error message when failed
    CompletedAt time.Time // zero when not completed
}

func (e *Executor) Start(ctx context.Context, req *functionv1.InvokeRequest, localAddr string) (string, bool) {
    e.mu.Lock()
    // prune expired idempotency entries
    now := time.Now()
    for k, v := range e.idem { if now.After(v.expireAt) { delete(e.idem, k) } }
    if req.GetIdempotencyKey() != "" {
        if ent, ok := e.idem[req.GetIdempotencyKey()]; ok && now.Before(ent.expireAt) {
            e.mu.Unlock()
            return ent.jobID, true
        }
    }
    jobID := randHex(12)
    ch := make(chan *functionv1.JobEvent, 16)
    e.ch[jobID] = ch
    if req.GetIdempotencyKey() != "" { e.idem[req.GetIdempotencyKey()] = idemEntry{jobID: jobID, expireAt: now.Add(idemWindow)} }
    e.mu.Unlock()

    go func() {
        defer close(ch)
        // simple staged progress
        emit := func(ev *functionv1.JobEvent) { select { case ch <- ev: default: } }
        emit(&functionv1.JobEvent{Type: "progress", Progress: 0})
        time.Sleep(200 * time.Millisecond)
        emit(&functionv1.JobEvent{Type: "progress", Progress: 20})

        // call local function via gRPC
        base := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json"))}
        opts := append(base, interceptors.Chain(nil)...)
        cc, err := grpc.Dial(localAddr, opts...)
        if err != nil { emit(&functionv1.JobEvent{Type: "error", Message: err.Error()}); return }
        cli := functionv1.NewFunctionServiceClient(cc)
        ctx2, cancel := context.WithCancel(ctx)
        e.mu.Lock(); e.cancel[jobID] = cancel; e.mu.Unlock()
        resp, err := cli.Invoke(ctx2, req)
        _ = cc.Close()
        if err != nil {
            emit(&functionv1.JobEvent{Type: "error", Message: err.Error()})
            e.mu.Lock(); e.results[jobID] = JobStatus{State: "error", Error: err.Error(), CompletedAt: time.Now()}; e.mu.Unlock()
            return
        }
        emit(&functionv1.JobEvent{Type: "progress", Progress: 90})
        time.Sleep(100 * time.Millisecond)
        emit(&functionv1.JobEvent{Type: "done", Payload: resp.GetPayload()})
        e.mu.Lock(); e.results[jobID] = JobStatus{State: "done", Payload: resp.GetPayload(), CompletedAt: time.Now()}; e.mu.Unlock()
    }()

    return jobID, false
}

func (e *Executor) Stream(jobID string) (<-chan *functionv1.JobEvent, bool) {
    e.mu.RLock(); defer e.mu.RUnlock()
    ch, ok := e.ch[jobID]
    return ch, ok
}

func (e *Executor) Cancel(jobID string) bool {
    e.mu.Lock(); defer e.mu.Unlock()
    if c, ok := e.cancel[jobID]; ok {
        c()
        delete(e.cancel, jobID)
        return true
    }
    return false
}

// Status returns current/final job status if known.
func (e *Executor) Status(jobID string) (JobStatus, bool) {
    e.mu.RLock(); defer e.mu.RUnlock()
    if st, ok := e.results[jobID]; ok { return st, true }
    if _, ok := e.ch[jobID]; ok { return JobStatus{State: "running"}, true }
    return JobStatus{}, false
}

func randHex(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}
