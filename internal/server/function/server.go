package function

import (
    "context"
    "errors"
    "fmt"
    "log"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    "github.com/your-org/croupier/internal/jobs"
    "github.com/your-org/croupier/internal/server/registry"

    "google.golang.org/grpc"
)

// Server implements FunctionService at Core side, routing calls to agents.
type Server struct {
    functionv1.UnimplementedFunctionServiceServer
    store *registry.Store
    jobs  *jobs.Router
}

func NewServer(store *registry.Store) *Server { return &Server{store: store, jobs: jobs.NewRouter()} }

func (s *Server) pickAgent(fid string) (*registry.AgentSession, error) {
    cands := s.store.AgentsForFunction(fid)
    if len(cands) == 0 { return nil, errors.New("no agent available") }
    // TODO: Load balancing policy; for now pick first
    return cands[0], nil
}

func (s *Server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    agent, err := s.pickAgent(req.GetFunctionId())
    if err != nil { return nil, err }
    cc, err := grpc.Dial(agent.RPCAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil { return nil, fmt.Errorf("dial agent %s: %w", agent.AgentID, err) }
    defer cc.Close()
    cli := functionv1.NewFunctionServiceClient(cc)
    log.Printf("routing invoke %s to agent %s@%s", req.GetFunctionId(), agent.AgentID, agent.RPCAddr)
    return cli.Invoke(ctx, req)
}

func (s *Server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    agent, err := s.pickAgent(req.GetFunctionId())
    if err != nil { return nil, err }
    cc, err := grpc.Dial(agent.RPCAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil { return nil, fmt.Errorf("dial agent %s: %w", agent.AgentID, err) }
    defer cc.Close()
    cli := functionv1.NewFunctionServiceClient(cc)
    log.Printf("routing start-job %s to agent %s@%s", req.GetFunctionId(), agent.AgentID, agent.RPCAddr)
    resp, err := cli.StartJob(ctx, req)
    if err == nil {
        s.jobs.Set(resp.GetJobId(), agent.RPCAddr)
    }
    return resp, err
}

func (s *Server) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
    rpcAddr, ok := s.jobs.Get(req.GetJobId())
    if !ok { return errors.New("unknown job") }
    cc, err := grpc.Dial(rpcAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil { return fmt.Errorf("dial agent: %w", err) }
    defer cc.Close()
    cli := functionv1.NewFunctionServiceClient(cc)

    // Fan-out events from agent to caller
    agentStream, err := cli.StreamJob(stream.Context(), req)
    if err != nil { return err }
    for {
        ev, err := agentStream.Recv()
        if err != nil { return err }
        if err := stream.Send(ev); err != nil { return err }
        if ev.GetType() == "done" || ev.GetType() == "error" { return nil }
    }
}

func (s *Server) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    rpcAddr, ok := s.jobs.Get(req.GetJobId())
    if !ok { return nil, errors.New("unknown job") }
    cc, err := grpc.Dial(rpcAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil { return nil, fmt.Errorf("dial agent: %w", err) }
    defer cc.Close()
    cli := functionv1.NewFunctionServiceClient(cc)
    return cli.CancelJob(ctx, req)
}

// Implement client-like helper to satisfy httpserver.FunctionInvoker
func (s *Server) StreamJobClient(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
    // not used; httpserver expects StreamJob, not StreamJobClient
    return nil, errors.New("not implemented")
}

// Compile-time check
var _ interface{ 
    Invoke(context.Context, *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
    StartJob(context.Context, *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
    StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
    CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
} = (*clientAdapter)(nil)

// clientAdapter wraps Server to expose client-style StreamJob for httpserver.
type clientAdapter struct{ s *Server }

func (a *clientAdapter) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    return a.s.Invoke(ctx, req)
}
func (a *clientAdapter) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    return a.s.StartJob(ctx, req)
}
func (a *clientAdapter) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
    // create a client by dialing the chosen agent and delegating stream to it
    rpcAddr, ok := a.s.jobs.Get(req.GetJobId())
    if !ok { return nil, errors.New("unknown job") }
    cc, err := grpc.Dial(rpcAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
    if err != nil { return nil, err }
    // Note: caller must drain stream and connection will close when GC'ed; here we don't retain cc reference
    cli := functionv1.NewFunctionServiceClient(cc)
    return cli.StreamJob(ctx, req)
}
func (a *clientAdapter) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    return a.s.CancelJob(ctx, req)
}

func NewClientAdapter(s *Server) *clientAdapter { return &clientAdapter{s: s} }
