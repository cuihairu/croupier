package jobs

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    "github.com/your-org/croupier/internal/transport/interceptors"
    "google.golang.org/grpc"
)

type Executor struct {
    mu    sync.RWMutex
    ch    map[string]chan *functionv1.JobEvent
    idem  map[string]string // idempotency key -> job id
    cancel map[string]context.CancelFunc
}

func NewExecutor() *Executor { return &Executor{ch: map[string]chan *functionv1.JobEvent{}, idem: map[string]string{}, cancel: map[string]context.CancelFunc{}} }

func (e *Executor) Start(ctx context.Context, req *functionv1.InvokeRequest, localAddr string) (string, bool) {
    e.mu.Lock()
    if req.GetIdempotencyKey() != "" {
        if j, ok := e.idem[req.GetIdempotencyKey()]; ok {
            e.mu.Unlock()
            return j, true
        }
    }
    jobID := randHex(12)
    ch := make(chan *functionv1.JobEvent, 16)
    e.ch[jobID] = ch
    if req.GetIdempotencyKey() != "" { e.idem[req.GetIdempotencyKey()] = jobID }
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
        if err != nil { emit(&functionv1.JobEvent{Type: "error", Message: err.Error()}); return }
        emit(&functionv1.JobEvent{Type: "progress", Progress: 90})
        time.Sleep(100 * time.Millisecond)
        emit(&functionv1.JobEvent{Type: "done", Payload: resp.GetPayload()})
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

func randHex(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}
