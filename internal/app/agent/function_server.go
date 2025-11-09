package agent

import (
    "context"
    "time"
    agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// FunctionServer forwards protobuf calls to local game servers that expose FunctionService.
type FunctionServer struct{
    functionv1.UnimplementedFunctionServiceServer
    store *agentlocal.LocalStore
    jobs  *jobIndex
}

// pickInstance returns the first available instance for a function id.
func (s *FunctionServer) pickInstance(fid string) (addr string, ok bool) {
    if s.store == nil || fid == "" { return "", false }
    snap := s.store.List()
    arr := snap[fid]
    if len(arr) == 0 { return "", false }
    return arr[0].Addr, true
}

func (s *FunctionServer) dial(addr string) (*grpc.ClientConn, functionv1.FunctionServiceClient, error) {
    cc, err := grpc.Dial(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
    )
    if err != nil { return nil, nil, err }
    return cc, functionv1.NewFunctionServiceClient(cc), nil
}

func (s *FunctionServer) Invoke(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    addr, ok := s.pickInstance(in.GetFunctionId())
    if !ok { return &functionv1.InvokeResponse{Payload: nil}, nil }
    cc, cli, err := s.dial(addr)
    if err != nil { return &functionv1.InvokeResponse{Payload: nil}, nil }
    defer cc.Close()
    c2, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    return cli.Invoke(c2, in)
}

func (s *FunctionServer) StartJob(ctx context.Context, in *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    addr, ok := s.pickInstance(in.GetFunctionId())
    if !ok { return &functionv1.StartJobResponse{JobId: ""}, nil }
    cc, cli, err := s.dial(addr)
    if err != nil { return &functionv1.StartJobResponse{JobId: ""}, nil }
    defer cc.Close()
    c2, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    resp, err := cli.StartJob(c2, in)
    if err == nil && resp != nil && resp.GetJobId() != "" && s.jobs != nil {
        s.jobs.Set(resp.GetJobId(), addr)
    }
    return resp, err
}

func (s *FunctionServer) CancelJob(ctx context.Context, in *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    if in == nil || in.GetJobId() == "" || s.jobs == nil { return &functionv1.StartJobResponse{JobId: in.GetJobId()}, nil }
    if addr, ok := s.jobs.Get(in.GetJobId()); ok {
        cc, cli, err := s.dial(addr)
        if err == nil {
            defer cc.Close()
            c2, cancel := context.WithTimeout(ctx, 3*time.Second)
            defer cancel()
            resp, err2 := cli.CancelJob(c2, in)
            // best-effort: remove mapping after cancel
            s.jobs.Delete(in.GetJobId())
            if err2 == nil { return resp, nil }
        }
    }
    // fallback: acknowledge cancel
    return &functionv1.StartJobResponse{JobId: in.GetJobId()}, nil
}
