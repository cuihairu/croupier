package function

import (
    "context"
    "fmt"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    "github.com/your-org/croupier/internal/agent/registry"
    "github.com/your-org/croupier/internal/agent/jobs"
    "github.com/your-org/croupier/internal/transport/interceptors"

    "google.golang.org/grpc"
)

// Server implements FunctionService at Agent side.
// For now it returns stub responses and simulates progress.
type Server struct {
    functionv1.UnimplementedFunctionServiceServer
    store *registry.LocalStore
    exec  *jobs.Executor
}

func NewServer(store *registry.LocalStore, exec *jobs.Executor) *Server { return &Server{store: store, exec: exec} }

func (s *Server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    // route to local game server based on function id
    entry, ok := s.store.Get(req.GetFunctionId())
    if !ok { return nil, fmt.Errorf("no local handler for %s", req.GetFunctionId()) }
    // best-effort trace log
    if req.Metadata != nil {
        // defer print to avoid noisy logs; a structured logger would be preferred
        _ = req.Metadata["trace_id"]
    }
    base := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json"))}
    opts := append(base, interceptors.Chain(nil)...)
    cc, err := grpc.Dial(entry.Addr, opts...)
    if err != nil { return nil, err }
    defer cc.Close()
    cli := functionv1.NewFunctionServiceClient(cc)
    return cli.Invoke(ctx, req)
}

func (s *Server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    entry, ok := s.store.Get(req.GetFunctionId())
    if !ok { return nil, fmt.Errorf("no local handler for %s", req.GetFunctionId()) }
    // Start job executor that wraps Invoke into async with progress
    jobID, existed := s.exec.Start(ctx, req, entry.Addr)
    if existed { return &functionv1.StartJobResponse{JobId: jobID}, nil }
    return &functionv1.StartJobResponse{JobId: jobID}, nil
}

func (s *Server) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
    ch, ok := s.exec.Stream(req.GetJobId())
    if !ok { return fmt.Errorf("unknown job") }
    for ev := range ch {
        if err := stream.Send(ev); err != nil { return err }
        if ev.GetType() == "done" || ev.GetType() == "error" { return nil }
    }
    return nil
}

func (s *Server) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    if ok := s.exec.Cancel(req.GetJobId()); ok {
        return &functionv1.StartJobResponse{JobId: req.GetJobId()}, nil
    }
    return &functionv1.StartJobResponse{JobId: req.GetJobId()}, nil
}
