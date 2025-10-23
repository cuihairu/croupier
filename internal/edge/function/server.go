package function

import (
    "context"
    "errors"
    "fmt"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    "github.com/your-org/croupier/internal/edge/tunnel"
    "github.com/your-org/croupier/internal/server/registry"
    function "github.com/your-org/croupier/internal/server/function"
)

// EdgeServer forwards FunctionService calls via tunnel when possible, else falls back to dialing RPCAddr.
type EdgeServer struct {
    functionv1.UnimplementedFunctionServiceServer
    store *registry.Store
    tun   *tunnel.Server
}

func NewEdgeServer(store *registry.Store, tun *tunnel.Server) *EdgeServer { return &EdgeServer{store: store, tun: tun} }

func (s *EdgeServer) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    var gameID string
    if req.Metadata != nil { gameID = req.Metadata["game_id"] }
    cands := s.store.AgentsForFunctionScoped(gameID, req.GetFunctionId(), true)
    if len(cands) == 0 { return nil, errors.New("no agent available") }
    // prefer first candidate; invoke via tunnel
    agent := cands[0]
    // try tunnel
    rid := req.GetIdempotencyKey()
    if rid == "" { rid = fmt.Sprintf("rid-%s", agent.AgentID) }
    if s.tun != nil {
        res, err := s.tun.InvokeViaTunnel(agent.AgentID, rid, req.GetFunctionId(), req.GetIdempotencyKey(), req.GetPayload(), req.Metadata)
        if err == nil {
            return &functionv1.InvokeResponse{Payload: res.Payload}, nil
        }
        // fallback
    }
    // fallback to legacy dialing: reuse existing server impl by composing
    legacy := function.NewServer(s.store)
    return legacy.Invoke(ctx, req)
}

func (s *EdgeServer) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    var gameID string
    if req.Metadata != nil { gameID = req.Metadata["game_id"] }
    cands := s.store.AgentsForFunctionScoped(gameID, req.GetFunctionId(), true)
    if len(cands) == 0 { return nil, errors.New("no agent available") }
    agent := cands[0]
    rid := req.GetIdempotencyKey()
    if rid == "" { rid = fmt.Sprintf("rid-%s", agent.AgentID) }
    if s.tun != nil {
        jobID, err := s.tun.StartJobViaTunnel(agent.AgentID, rid, req.GetFunctionId(), req.GetIdempotencyKey(), req.GetPayload(), req.Metadata)
        if err == nil { return &functionv1.StartJobResponse{JobId: jobID}, nil }
    }
    legacy := function.NewServer(s.store)
    return legacy.StartJob(ctx, req)
}

func (s *EdgeServer) StreamJob(req *functionv1.JobStreamRequest, srv functionv1.FunctionService_StreamJobServer) error {
    if s.tun != nil {
        ch := s.tun.SubscribeJob(req.GetJobId())
        for ev := range ch {
            if err := srv.Send(ev); err != nil { return err }
            if ev.GetType() == "done" || ev.GetType() == "error" { return nil }
        }
        return nil
    }
    legacy := function.NewServer(s.store)
    return legacy.StreamJob(req, srv)
}

func (s *EdgeServer) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    if s.tun != nil {
        _ = s.tun.CancelJobViaTunnel(req.GetJobId())
    }
    legacy := function.NewServer(s.store)
    return legacy.CancelJob(ctx, req)
}
